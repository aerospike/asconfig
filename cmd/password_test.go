//go:build unit

package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aerospike/tools-common-go/config"
	"github.com/aerospike/tools-common-go/flags"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func TestGenerateReadsPasswordSources(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}

	t.Setenv("ASCONFIG_TEST_PASSWORD", "admin")

	keyFile := filepath.Join(t.TempDir(), "key.pem")
	if err := os.WriteFile(keyFile, []byte("key"), outputFilePermissions); err != nil {
		t.Fatal(err)
	}

	tests := map[string]struct {
		want     error
		args     []string
		contains []string
	}{
		"env source": {
			args: []string{"-U", "admin", "-P", "env:ASCONFIG_TEST_PASSWORD"},
			want: errUnableToGenerateConfigFile,
		},
		"env without a name": {
			args:     []string{"-U", "admin", "-P", "env:"},
			want:     errUnableToResolveSecrets,
			contains: []string{"--password: env: requires a variable name"},
		},
		"63 bytes": {
			args: []string{"-U", "admin", "-P", strings.Repeat("p", 63)},
			want: errUnableToGenerateConfigFile,
		},
		"64 bytes": {
			args:     []string{"-U", "admin", "-P", strings.Repeat("p", 64)},
			want:     errUnableToResolveSecrets,
			contains: []string{"--password: password is longer than 63 bytes"},
		},
		"PKI does not read the password": {
			args: []string{"-U", "admin", "--auth", "pki", "-P", "env:ASCONFIG_TEST_UNSET"},
			want: errUnableToGenerateConfigFile,
		},
		"key file password": {
			args:     []string{"--tls-keyfile", keyFile, "--tls-keyfile-password", "file:"},
			want:     errUnableToResolveSecrets,
			contains: []string{"--tls-keyfile-password: file: requires a path"},
		},
		"key file password without a key file": {
			args: []string{"--tls-keyfile-password", "file:"},
			want: errUnableToGenerateConfigFile,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			cmd := newGenerateCmd()
			if err := cmd.ParseFlags(append([]string{"-h", unreachableAddr}, tt.args...)); err != nil {
				t.Fatal(err)
			}

			requireErr(t, cmd.RunE(cmd, nil), tt.want, tt.contains...)
		})
	}
}

func TestGenerateTrimsKeyFilePassword(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}

	t.Setenv("ASCONFIG_TEST_KEY_PASSWORD", "key-pass")

	keyFile := filepath.Join(t.TempDir(), "key.pem")
	if err := os.WriteFile(keyFile, []byte("key"), outputFilePermissions); err != nil {
		t.Fatal(err)
	}

	cmd := newGenerateCmd()
	if err := cmd.ParseFlags([]string{
		"-h", unreachableAddr, "--tls-keyfile", keyFile, "--tls-keyfile-password", " env:ASCONFIG_TEST_KEY_PASSWORD ",
	}); err != nil {
		t.Fatal(err)
	}

	requireErr(t, cmd.RunE(cmd, nil), errUnableToGenerateConfigFile)

	if got := cmd.Flags().Lookup("tls-keyfile-password").Value.String(); got != "key-pass" {
		t.Fatalf("expected the key file password %q, got %q", "key-pass", got)
	}
}

func TestGeneratePasswordWithoutUser(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}

	tests := map[string]struct {
		warning string
		args    []string
	}{
		"source": {
			args:    []string{"-P", "file:/missing"},
			warning: "--password: ignored because no user is set",
		},
		"literal": {args: []string{"-P", "admin"}},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var logs bytes.Buffer

			logger.SetOutput(&logs)
			t.Cleanup(func() { logger.SetOutput(os.Stderr) })

			cmd := newGenerateCmd()
			if err := cmd.ParseFlags(append([]string{"-h", unreachableAddr}, tt.args...)); err != nil {
				t.Fatal(err)
			}

			requireErr(t, cmd.RunE(cmd, nil), errUnableToGenerateConfigFile)

			hasWarning := strings.Contains(logs.String(), "level=warning")
			if tt.warning == "" && hasWarning {
				t.Fatalf("expected no warning, got %s", logs.String())
			}

			if tt.warning != "" && (!hasWarning || !strings.Contains(logs.String(), tt.warning)) {
				t.Fatalf("expected the warning %q, got %s", tt.warning, logs.String())
			}
		})
	}
}

func TestGenerateSecretPasswordWithoutUser(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}

	tests := map[string]struct {
		config  string
		warning string
		args    []string
	}{
		"command line": {
			args:    []string{"-P", "secrets:asconfig:password"},
			warning: "--password: ignored because no user is set",
		},
		"config file": {
			config:  "[cluster]\npassword = \"secrets:asconfig:password\"\n",
			warning: "password in config file: ignored because no user is set",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var logs bytes.Buffer

			logger.SetOutput(&logs)
			t.Cleanup(func() { logger.SetOutput(os.Stderr) })

			config.Reset()
			t.Cleanup(config.Reset)

			configFile := filepath.Join(t.TempDir(), "astools.conf")
			if err := os.WriteFile(configFile, []byte(tt.config), outputFilePermissions); err != nil {
				t.Fatal(err)
			}

			*cfFileFlags = flags.ConfFileFlags{}

			root := NewRootCmd()
			root.AddCommand(newGenerateCmd())
			root.SetArgs(append([]string{
				"generate", "--config-file", configFile, "-h", unreachableAddr, "--sa-address", unreachableAddr,
			}, tt.args...))

			requireErr(t, root.Execute(), errUnableToGenerateConfigFile)

			if !strings.Contains(logs.String(), "level=warning") || !strings.Contains(logs.String(), tt.warning) {
				t.Fatalf("expected the warning %q, got %s", tt.warning, logs.String())
			}
		})
	}
}

func TestGeneratePasswordFromConfigFile(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}

	tests := map[string]struct {
		want     error
		config   string
		args     []string
		contains []string
	}{
		"config file label": {
			config:   "[cluster]\nhost = \"" + unreachableAddr + "\"\nuser = \"admin\"\npassword = \"file:\"\n",
			want:     errUnableToResolveSecrets,
			contains: []string{"password in config file: file: requires a path"},
		},
		"command line beats the config file": {
			config: "[cluster]\nhost = \"" + unreachableAddr + "\"\nuser = \"admin\"\npassword = \"file:\"\n",
			args:   []string{"-P", "admin"},
			want:   errUnableToGenerateConfigFile,
		},
		"instance section": {
			config: "[cluster]\nhost = \"" + unreachableAddr + "\"\nuser = \"admin\"\npassword = \"admin\"\n\n" +
				"[cluster_x]\nhost = \"" + unreachableAddr + "\"\nuser = \"admin\"\npassword = \"b64:\"\n",
			args:     []string{"--instance", "x"},
			want:     errUnableToResolveSecrets,
			contains: []string{"password in config file: password from b64: value is empty"},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			config.Reset()
			t.Cleanup(config.Reset)

			configFile := filepath.Join(t.TempDir(), "astools.conf")
			if err := os.WriteFile(configFile, []byte(tt.config), outputFilePermissions); err != nil {
				t.Fatal(err)
			}

			*cfFileFlags = flags.ConfFileFlags{}

			root := NewRootCmd()
			root.AddCommand(newGenerateCmd())
			root.SetArgs(append([]string{"generate", "--config-file", configFile}, tt.args...))

			requireErr(t, root.Execute(), tt.want, tt.contains...)
		})
	}
}

func TestPasswordSourceErrorsNameOnlyTheOption(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}

	t.Setenv("ASCONFIG_TEST_UNSET", "")

	tests := map[string]struct {
		newCmd func() *cobra.Command
		args   []string
	}{
		"generate":    {newCmd: newGenerateCmd},
		"diff server": {newCmd: newDiffServerCmd, args: []string{"../testdata/expected/all_flash_cluster_cr.conf"}},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			cmd := tt.newCmd()
			if err := cmd.ParseFlags(
				[]string{"-h", unreachableAddr, "-U", "admin", "-P", "env:ASCONFIG_TEST_UNSET"},
			); err != nil {
				t.Fatal(err)
			}

			err := cmd.RunE(cmd, tt.args)
			requireErr(t, err, errUnableToResolveSecrets,
				"--password: environment variable ASCONFIG_TEST_UNSET is not set or empty")

			if strings.Contains(err.Error(), "secrets") {
				t.Fatalf("expected an error that names only the option, got %v", err)
			}
		})
	}
}

func TestPasswordHelp(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}

	commands := map[string]*cobra.Command{
		"generate":    newGenerateCmd(),
		"diff server": newDiffServerCmd(),
	}

	lines := []string{
		"Password used to authenticate with cluster. Default: none",
		`4) File: 'file:<PATH>', the whole file less one trailing '\n' or`,
		"and one trailing newline is dropped after decoding. A 'b64:' value",
		"not PKI. It must be at most 63 bytes.",
		"Password to load protected tls-keyfile.",
		"4) File: 'file:<PATH>', the first line of the file",
		"Surrounding whitespace is trimmed before the form is read. A",
		"password with surrounding whitespace must be given as 'b64:'.",
		"The password is read only when a key file is set. It must be at",
		"most 1024 bytes.",
		"A value fetched with 'secrets:' is used as-is.",
	}

	for name, cmd := range commands {
		usage := cmd.Flags().FlagUsages()

		for _, line := range lines {
			if !strings.Contains(usage, line+"\n") {
				t.Errorf("%s: help is missing the line %q", name, line)
			}
		}
	}
}

func flagHelp(t *testing.T, cmd *cobra.Command, name string) string {
	t.Helper()

	flag := cmd.Flags().Lookup(name)
	if flag == nil {
		t.Fatalf("missing --%s", name)
	}

	fs := pflag.NewFlagSet(name, pflag.ContinueOnError)
	fs.AddFlag(flag)

	return strings.Join(strings.Fields(fs.FlagUsages()), " ")
}

func TestPasswordHelpMatchesTheOtherTools(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}

	// The canonical text, less the prompt sentences: asconfig has no prompt.
	help := map[string]string{
		"password": `Password used to authenticate with cluster. Default: none
			It can be one of the following:
			1) Environment variable: 'env:<VAR>', used exactly as set
			2) Base64 encoded environment variable: 'env-b64:<VAR>'
			3) Base64 encoded string: 'b64:<BASE64>'
			4) File: 'file:<PATH>', the whole file less one trailing '\n' or
			   '\r\n'. PATH should name a regular file.
			5) Aerospike Secret Agent: 'secrets:<resource>:<key>'
			6) String: 'PASSWORD'
			Prefixes are case-sensitive. A base64 value may be line wrapped,
			and one trailing newline is dropped after decoding. A 'b64:' value
			on the command line is as visible as a literal password.
			A password that itself begins with 'env:', 'env-b64:', 'b64:',
			'file:' or 'secrets:' must be given as 'b64:'.
			The password is read only when a user is set and the auth mode is
			not PKI. It must be at most 63 bytes.
			A value fetched with 'secrets:' is used as-is.`,
		"tls-keyfile-password": `Password to load protected tls-keyfile.
			It can be one of the following:
			1) Environment variable: 'env:<VAR>', used exactly as set
			2) Base64 encoded environment variable: 'env-b64:<VAR>'
			3) Base64 encoded string: 'b64:<BASE64>'
			4) File: 'file:<PATH>', the first line of the file
			5) Aerospike Secret Agent: 'secrets:<resource>:<key>'
			6) String: 'PASSWORD'
			Prefixes are case-sensitive. A base64 value may be line wrapped,
			and one trailing newline is dropped after decoding.
			A password that itself begins with 'env:', 'env-b64:', 'b64:',
			'file:' or 'secrets:' must be given as 'b64:'.
			Surrounding whitespace is trimmed before the form is read. A
			password with surrounding whitespace must be given as 'b64:'.
			The password is read only when a key file is set. It must be at
			most 1024 bytes.
			A value fetched with 'secrets:' is used as-is.
			Default: none`,
	}

	commands := map[string]*cobra.Command{
		"generate":    newGenerateCmd(),
		"diff server": newDiffServerCmd(),
	}

	for cmdName, cmd := range commands {
		for name, text := range help {
			usage := flagHelp(t, cmd, name)

			if want := strings.Join(strings.Fields(text), " "); !strings.Contains(usage, want) {
				t.Errorf("%s: --%s help is missing %q, got %q", cmdName, name, want, usage)
			}
		}
	}
}
