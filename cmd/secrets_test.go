//go:build unit

package cmd

import (
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aerospike/tools-common-go/config"
	"github.com/aerospike/tools-common-go/flags"
	"github.com/aerospike/tools-common-go/secretagent/secretagenttest"
	"github.com/spf13/cobra"
)

// unreachableAddr is never listened on, so Aerospike and Secret Agent
// connections to it fail fast.
const unreachableAddr = "127.0.0.1:1"

func newSecretsTestAgent(t *testing.T) *secretagenttest.Server {
	t.Helper()

	return secretagenttest.NewServer(t, secretagenttest.Secrets{
		"asconfig": {"password": base64.StdEncoding.EncodeToString([]byte("admin"))},
	})
}

func requireErr(t *testing.T, err, want error, contains ...string) {
	t.Helper()

	if !errors.Is(err, want) {
		t.Fatalf("expected %v, got %v", want, err)
	}

	for _, s := range contains {
		if !strings.Contains(err.Error(), s) {
			t.Fatalf("expected error to contain %q, got %v", s, err)
		}
	}
}

func TestSecretAgentFlagsRegistered(t *testing.T) {
	commands := map[string]*cobra.Command{
		"generate":    newGenerateCmd(),
		"diff server": newDiffServerCmd(),
	}

	for name, cmd := range commands {
		for _, flag := range []string{"sa-address", "sa-port", "sa-timeout", "sa-ca-file"} {
			if cmd.Flags().Lookup(flag) == nil {
				t.Errorf("%s: missing --%s", name, flag)
			}
		}
	}
}

func TestGenerateResolvesSecrets(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}

	agent := newSecretsTestAgent(t)
	base := []string{"-h", unreachableAddr, "-U", "admin"}

	tests := map[string]struct {
		want     error
		password string
		args     []string
		contains []string
	}{
		"resolved, then the server is unreachable": {
			args:     []string{"-P", "secrets:asconfig:password", "--sa-address", agent.Addr()},
			want:     errUnableToGenerateConfigFile,
			password: "admin",
		},
		"secret agent address after the password": {
			args:     []string{"--sa-address", agent.Addr(), "-P", "secrets:asconfig:password"},
			want:     errUnableToGenerateConfigFile,
			password: "admin",
		},
		"missing key": {
			args:     []string{"-P", "secrets:asconfig:missing", "--sa-address", agent.Addr()},
			want:     errUnableToResolveSecrets,
			contains: []string{"--password", "secrets:asconfig:missing"},
		},
		"agent unreachable": {
			args:     []string{"-P", "secrets:asconfig:password", "--sa-address", unreachableAddr},
			want:     errUnableToResolveSecrets,
			contains: []string{"--password", "secrets:asconfig:password", unreachableAddr},
		},
		"user secret": {
			args:     []string{"-U", "secrets:asconfig:missing", "-P", "admin", "--sa-address", agent.Addr()},
			want:     errUnableToResolveSecrets,
			contains: []string{"--user", "secrets:asconfig:missing"},
		},
		"b64 password that decodes to secrets: stays literal": {
			args: []string{
				"-P", "b64:" + base64.StdEncoding.EncodeToString([]byte("secrets:asconfig:password")),
				"--sa-address", unreachableAddr,
			},
			want: errUnableToGenerateConfigFile,
		},
		"invalid port without a secret is ignored": {
			args: []string{"-P", "admin", "--sa-port", "0"},
			want: errUnableToGenerateConfigFile,
		},
		"invalid port with a secret": {
			args:     []string{"-P", "secrets:asconfig:password", "--sa-port", "0"},
			want:     errUnableToResolveSecrets,
			contains: []string{"--sa-port: invalid value 0"},
		},
		"invalid timeout with a secret": {
			args:     []string{"-P", "secrets:asconfig:password", "--sa-address", agent.Addr(), "--sa-timeout", "0"},
			want:     errUnableToResolveSecrets,
			contains: []string{"--sa-timeout: invalid value 0"},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			cmd := newGenerateCmd()
			if err := cmd.ParseFlags(append(base, tt.args...)); err != nil {
				t.Fatal(err)
			}

			requireErr(t, cmd.RunE(cmd, nil), tt.want, tt.contains...)

			if tt.password != "" {
				if got := cmd.Flags().Lookup("password").Value.String(); got != tt.password {
					t.Fatalf("expected the fetched password %q, got %q", tt.password, got)
				}
			}
		})
	}
}

func TestGenerateResolvesSecretsFromConfigFile(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}

	agent := newSecretsTestAgent(t)
	tlsAgent := secretagenttest.NewTLSServer(t, secretagenttest.Secrets{
		"asconfig": {"password": base64.StdEncoding.EncodeToString([]byte("admin"))},
	})

	tests := map[string]struct {
		want     error
		config   string
		args     []string
		contains []string
	}{
		"C tools spelling sa-cafile": {
			config: `[cluster]
host = "` + unreachableAddr + `"
user = "admin"
password = "secrets:asconfig:password"

[secret-agent]
sa-address = "` + tlsAgent.Addr() + `"
sa-cafile = "` + tlsAgent.CAFile() + `"
`,
			want: errUnableToGenerateConfigFile,
		},
		"secret-agent section": {
			config: `[cluster]
host = "` + unreachableAddr + `"
user = "admin"
password = "secrets:asconfig:password"

[secret-agent]
sa-address = "` + agent.Addr() + `"
`,
			want: errUnableToGenerateConfigFile,
		},
		"instance falls back to secret-agent section": {
			config: `[cluster_a]
host = "` + unreachableAddr + `"
user = "admin"
password = "secrets:asconfig:password"

[secret-agent]
sa-address = "` + agent.Addr() + `"
`,
			args: []string{"--instance", "a"},
			want: errUnableToGenerateConfigFile,
		},
		"instance section replaces secret-agent section": {
			config: `[cluster_a]
host = "` + unreachableAddr + `"
user = "admin"
password = "secrets:asconfig:password"

[secret-agent]
sa-address = "` + agent.Addr() + `"

[secret-agent_a]
sa-address = "` + unreachableAddr + `"
`,
			args:     []string{"--instance", "a"},
			want:     errUnableToResolveSecrets,
			contains: []string{"--password", "secrets:asconfig:password", unreachableAddr},
		},
		"command line beats config file": {
			config: `[cluster]
host = "` + unreachableAddr + `"
user = "admin"
password = "secrets:asconfig:password"

[secret-agent]
sa-address = "` + unreachableAddr + `"
`,
			args: []string{"--sa-address", agent.Addr()},
			want: errUnableToGenerateConfigFile,
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

			rootCmd := NewRootCmd()
			rootCmd.AddCommand(newGenerateCmd())
			rootCmd.SetArgs(append([]string{"generate", "--config-file", configFile}, tt.args...))

			requireErr(t, rootCmd.Execute(), tt.want, tt.contains...)
		})
	}
}
