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
		for _, flag := range []string{"sa-address", "sa-port", "sa-timeout", "sa-cafile"} {
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
		args     []string
		contains []string
	}{
		"resolved, then the server is unreachable": {
			args: []string{"-P", "secrets:asconfig:password", "--sa-address", agent.Addr()},
			want: errUnableToGenerateConfigFile,
		},
		"secret agent address after the password": {
			args: []string{"--sa-address", agent.Addr(), "-P", "secrets:asconfig:password"},
			want: errUnableToGenerateConfigFile,
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
		"invalid port": {
			args:     []string{"-P", "admin", "--sa-port", "0"},
			want:     errUnableToResolveSecrets,
			contains: []string{"--sa-port: invalid value 0"},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			cmd := newGenerateCmd()
			if err := cmd.ParseFlags(append(base, tt.args...)); err != nil {
				t.Fatal(err)
			}

			requireErr(t, cmd.RunE(cmd, nil), tt.want, tt.contains...)
		})
	}
}

func TestGenerateResolvesSecretsFromConfigFile(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}

	agent := newSecretsTestAgent(t)

	tests := map[string]struct {
		want     error
		config   string
		args     []string
		contains []string
	}{
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
