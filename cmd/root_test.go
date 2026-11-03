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
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/suite"
)

// Keeps the developer's ~/.aerospike/astools.conf out of every test.
func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "asconfig-home")
	if err != nil {
		panic(err)
	}

	_ = os.Setenv("HOME", home)

	system, err := os.MkdirTemp("", "asconfig-etc")
	if err != nil {
		panic(err)
	}

	config.SetSystemConfDir(system)

	code := m.Run()

	_ = os.RemoveAll(home)
	_ = os.RemoveAll(system)

	os.Exit(code)
}

func TestConfigFilesReadLikeCTools(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}

	agent := secretagenttest.NewTLSServer(t, secretagenttest.Secrets{
		"asconfig": {"password": base64.StdEncoding.EncodeToString([]byte("admin"))},
	})

	tests := map[string]struct {
		files    configFiles
		wantUser string
		wantPass string
		wantErr  string
	}{
		"--config-file is read after the home file, key by key": {
			files: configFiles{
				home: "[cluster]\nuser = \"home\"\npassword = \"admin\"\n",
				user: "[cluster]\nuser = \"admin\"\n",
			},
			wantUser: "admin",
			wantPass: "admin",
		},
		"the working directory file is not read": {
			files: configFiles{working: "[cluster]\nuser = \"working\"\npassword = \"admin\"\n"},
		},
		"a later sa-address keeps the sa-ca-file of an earlier file": {
			files: configFiles{
				home: "[secret-agent]\nsa-ca-file = \"" + agent.CAFile() + "\"\n",
				user: "[cluster]\nuser = \"admin\"\npassword = \"secrets:asconfig:password\"\n\n" +
					"[secret-agent]\nsa-address = \"" + agent.Addr() + "\"\n",
			},
			wantUser: "admin",
			wantPass: "admin",
		},
		"the [secret-agent] section of an earlier file is checked": {
			files: configFiles{
				home: "[secret-agent]\nsa-port = 0\n",
				user: "[cluster]\nuser = \"admin\"\npassword = \"admin\"\n",
			},
			wantErr: `invalid value "0" for sa-port in the [secret-agent] config section, expected an integer from 1 to 65535`,
		},
		"--only-config-file skips the home file": {
			files: configFiles{
				home: "[cluster]\nuser = \"home\"\npassword = \"admin\"\n",
				user: "[cluster]\nuser = \"admin\"\n",
				only: true,
			},
			wantUser: "admin",
		},
		"--only-config-file fetches with the [secret-agent] section of that file": {
			files: configFiles{
				home: "[secret-agent]\nsa-port = 0\n",
				user: "[cluster]\nuser = \"admin\"\npassword = \"secrets:asconfig:password\"\n\n" +
					"[secret-agent]\nsa-address = \"" + agent.Addr() + "\"\nsa-ca-file = \"" + agent.CAFile() + "\"\n",
				only: true,
			},
			wantUser: "admin",
			wantPass: "admin",
		},
		"--no-config-file reads no file": {
			files: configFiles{
				home: "[secret-agent]\nsa-port = 0\n",
				user: "[cluster\n",
				none: true,
			},
		},
		"--no-config-file with --only-config-file": {
			files: configFiles{
				user: "[cluster]\nuser = \"admin\"\n",
				only: true,
				none: true,
			},
			wantErr: "--no-config-file and only-config-file are mutually exclusive option. Please enable only one.",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			run := runGenerateWithConfigFiles(t, tt.files)

			if tt.wantErr != "" {
				requireErrText(t, run.err, tt.wantErr)

				if n := run.server.Conns(); n != 0 {
					t.Fatalf("expected no server connection, got %d", n)
				}

				return
			}

			requireErr(t, run.err, errUnableToGenerateConfigFile)

			if got := run.cmd.Flags().Lookup("user").Value.String(); got != tt.wantUser {
				t.Fatalf("expected the user %q, got %q", tt.wantUser, got)
			}

			requirePassword(t, run.cmd, tt.wantPass)

			if strings.Join(run.logged, "\n") != strings.Join(run.files, "\n") {
				t.Fatalf("expected the config files %q to be logged, got %q", run.files, run.logged)
			}
		})
	}
}

type configFilesRun struct {
	err    error
	cmd    *cobra.Command
	server *fakePeer
	files  []string
	logged []string
}

// configFiles holds the text of ~/.aerospike/astools.conf (home), of the
// --config-file or, with only, --only-config-file (user), and of
// ./astools.conf (working). none adds --no-config-file.
type configFiles struct {
	home    string
	user    string
	working string
	only    bool
	none    bool
}

// runGenerateWithConfigFiles runs generate against a fake server with the
// config files of files, each written only when not empty. run.files holds the
// files expected to be read.
func runGenerateWithConfigFiles(t *testing.T, files configFiles) configFilesRun {
	t.Helper()

	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Chdir(t.TempDir())

	run := configFilesRun{server: newFakeServer(t)}
	args := []string{"generate", "-h", run.server.Addr()}

	if files.home != "" {
		home := writeTestFile(t, filepath.Join(homeDir, ".aerospike", "astools.conf"), files.home)

		if !files.only && !files.none {
			run.files = append(run.files, home)
		}
	}

	if files.working != "" {
		writeTestFile(t, "astools.conf", files.working)
	}

	if files.user != "" {
		user := writeTestFile(t, filepath.Join(t.TempDir(), "user.conf"), files.user)

		option := "--config-file"
		if files.only {
			option = "--only-config-file"
		}

		args = append(args, option, user)

		if !files.none {
			run.files = append(run.files, user)
		}
	}

	if files.none {
		args = append(args, "--no-config-file")
	}

	config.Reset()
	t.Cleanup(config.Reset)

	*cfFileFlags = flags.ConfFileFlags{}

	root := NewRootCmd()
	root.AddCommand(newGenerateCmd())
	root.SetArgs(args)

	output := captureOutput(t, func() { run.err = root.Execute() })
	run.cmd, _, _ = root.Find([]string{"generate"})

	for _, line := range strings.Split(output, "\n") {
		if _, file, ok := strings.Cut(line, "Using config file: "); ok {
			run.logged = append(run.logged, strings.TrimSuffix(file, `"`))
		}
	}

	return run
}

func writeTestFile(t *testing.T, file, text string) string {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(file, []byte(text), outputFilePermissions); err != nil {
		t.Fatal(err)
	}

	return file
}

func TestConfigFileHelp(t *testing.T) {
	usage := strings.Join(strings.Fields(NewRootCmd().PersistentFlags().Lookup("config-file").Usage), " ")

	for _, want := range []string{"/etc/aerospike/astools.conf", "~/.aerospike/astools.conf"} {
		if !strings.Contains(usage, want) {
			t.Errorf("--config-file help is missing %s, got %q", want, usage)
		}
	}

	if strings.Contains(usage, "default is") {
		t.Errorf("--config-file help still describes a single default file: %q", usage)
	}
}

func TestConfigFileOptionsHelp(t *testing.T) {
	usage := strings.Join(strings.Fields(NewRootCmd().PersistentFlags().FlagUsages()), " ")

	for _, want := range []string{
		"--only-config-file string Read only this config file.",
		"--no-config-file Do not read any config file.",
		"cluster_a and secret-agent_a",
	} {
		if !strings.Contains(usage, want) {
			t.Errorf("config file help is missing %q, got %q", want, usage)
		}
	}

	if strings.Contains(usage, "uda_a") {
		t.Errorf("--instance help still names the retired uda_a section: %q", usage)
	}
}

func TestPersistentPreRunRootFlags(t *testing.T) {
	var testCases = []struct {
		flags          []string
		arguments      []string
		expectedErrors []error
	}{
		{
			flags:          []string{"-l", "info"},
			arguments:      []string{},
			expectedErrors: []error{nil},
		},
		{
			flags:          []string{"-l", "panic"},
			arguments:      []string{},
			expectedErrors: []error{nil},
		},
		{
			flags:     []string{"--log-level", "bad_level"},
			arguments: []string{},
			expectedErrors: []error{
				errInvalidLogLevel,
			},
		},
	}

	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}

	rootCmd := NewRootCmd()

	for _, tc := range testCases {
		t.Run(tc.flags[0], func(t *testing.T) {
			_ = rootCmd.ParseFlags(tc.flags)

			err := rootCmd.PersistentPreRunE(rootCmd, tc.arguments)
			for _, expectedErr := range tc.expectedErrors {
				if !errors.Is(err, expectedErr) {
					t.Errorf("%v\n actual err: %v\n is not expected err: %v", tc.flags, err, expectedErr)
				}
			}
		})
	}
}

const tomlConfigTxt = `
[group1]
str1 = "localhost:3000"
int1 = 3000
bool1 = true

[group2]
str2 = "localhost:4000"
int2 = 4000
bool2 = false
`

const yamlConfigTxt = `
group1:
  str1: "localhost:3000"
  int1:  3000
  bool1:  true

group2:
  str2: "localhost:4000"
  int2: 4000
  bool2: false
`

type RootTest struct {
	suite.Suite
}

func (testsuite *RootTest) TestPersistentPreRunRootInitConfig() {
	testCases := []struct {
		configFile    string
		configFileTxt string
		wantErr       string
	}{
		{
			configFile:    "test.conf",
			configFileTxt: tomlConfigTxt,
		},
		{
			configFile:    "test.yaml",
			configFileTxt: yamlConfigTxt,
			wantErr:       "failed to read config file test.yaml: toml: ",
		},
	}

	createCmd := func() (*cobra.Command, *pflag.FlagSet, *pflag.FlagSet) {
		rootCmd := NewRootCmd()
		subCmd := &cobra.Command{
			Use: "sub",
			Run: func(_ *cobra.Command, _ []string) {},
		}
		flagSet1 := &pflag.FlagSet{}
		flagSet2 := &pflag.FlagSet{}

		rootCmd.AddCommand(subCmd)
		flagSet1.String("str1", "str1", "string flag")
		flagSet1.Int("int1", 0, "int flag")
		flagSet1.Bool("bool1", false, "bool flag")
		flagSet2.String("str2", "str2", "string flag")
		flagSet2.Int("int2", 0, "int flag")
		flagSet2.Bool("bool2", false, "bool flag")
		config.BindPFlags(flagSet1, "group1")
		config.BindPFlags(flagSet2, "group2")
		subCmd.PersistentFlags().AddFlagSet(flagSet1)
		subCmd.PersistentFlags().AddFlagSet(flagSet2)

		return rootCmd, flagSet1, flagSet2
	}

	for _, tc := range testCases {
		testsuite.T().Run(tc.configFile, func(t *testing.T) {
			config.Reset()

			rootCmd, flagSet1, flagSet2 := createCmd()

			err := os.WriteFile(tc.configFile, []byte(tc.configFileTxt), outputFilePermissions)
			if err != nil {
				t.Fatalf("unable to write %s: %v", tc.configFile, err)
			}

			defer os.Remove(tc.configFile)

			rootCmd.SetArgs([]string{"sub", "--config-file", tc.configFile})

			err = rootCmd.Execute()
			if tc.wantErr != "" {
				testsuite.ErrorContains(err, tc.wantErr)
				return
			}

			if err != nil {
				testsuite.FailNow("unexpected error", err)
			}

			str1, err := flagSet1.GetString("str1")
			testsuite.Require().NoError(err)
			testsuite.Equal("localhost:3000", str1)

			int1, err := flagSet1.GetInt("int1")
			testsuite.Require().NoError(err)
			testsuite.Equal(3000, int1)

			bool1, err := flagSet1.GetBool("bool1")
			testsuite.Require().NoError(err)
			testsuite.True(bool1)

			str2, err := flagSet2.GetString("str2")
			testsuite.Require().NoError(err)
			testsuite.Equal("localhost:4000", str2)

			int2, err := flagSet2.GetInt("int2")
			testsuite.Require().NoError(err)
			testsuite.Equal(4000, int2)

			bool2, err := flagSet2.GetBool("bool2")
			testsuite.Require().NoError(err)
			testsuite.False(bool2)
		})
	}
}

func TestRunTestSuite(t *testing.T) {
	suite.Run(t, new(RootTest))
}
