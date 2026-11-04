//go:build unit

package cmd

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/aerospike/tools-common-go/config"
	"github.com/aerospike/tools-common-go/flags"
	"github.com/aerospike/tools-common-go/secretagent/secretagenttest"
	"github.com/spf13/cobra"
)

const (
	secretAgentMagic    = 0x51dec1cc
	diffServerInput     = "../testdata/expected/all_flash_cluster_cr.conf"
	legacyCADeprecation = "Argument --sa-cafile is deprecated. Use --sa-ca-file instead."
)

// fakePeer counts the connections it accepts. As an agent it records each
// request and answers from secrets, keyed by "<resource>:<key>". As a server
// it closes every connection at once.
type fakePeer struct {
	listener net.Listener
	secrets  map[string]string
	requests []map[string]string
	conns    int
	mu       sync.Mutex
	agent    bool
}

func newFakeAgent(t *testing.T, secrets map[string]string) *fakePeer {
	t.Helper()

	return newFakePeer(t, &fakePeer{secrets: secrets, agent: true})
}

func newFakeServer(t *testing.T) *fakePeer {
	t.Helper()

	return newFakePeer(t, &fakePeer{})
}

func newFakePeer(t *testing.T, p *fakePeer) *fakePeer {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	p.listener = ln
	t.Cleanup(func() { _ = ln.Close() })

	go p.serve()

	return p
}

func (p *fakePeer) Addr() string {
	return p.listener.Addr().String()
}

func (p *fakePeer) Port() string {
	_, port, _ := net.SplitHostPort(p.Addr())
	return port
}

func (p *fakePeer) Conns() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.conns
}

func (p *fakePeer) Requests() []map[string]string {
	p.mu.Lock()
	defer p.mu.Unlock()

	return append([]map[string]string(nil), p.requests...)
}

func (p *fakePeer) serve() {
	for {
		conn, err := p.listener.Accept()
		if err != nil {
			return
		}

		p.mu.Lock()
		p.conns++
		p.mu.Unlock()

		if !p.agent {
			_ = conn.Close()
			continue
		}

		go p.answer(conn)
	}
}

func (p *fakePeer) answer(conn net.Conn) {
	defer conn.Close()

	header := make([]byte, 8)
	if _, err := io.ReadFull(conn, header); err != nil {
		return
	}

	size := binary.BigEndian.Uint32(header[4:])
	if binary.BigEndian.Uint32(header[:4]) != secretAgentMagic || size > 1<<20 {
		return
	}

	body := make([]byte, size)
	if _, err := io.ReadFull(conn, body); err != nil {
		return
	}

	var req map[string]string
	if err := json.Unmarshal(body, &req); err != nil {
		return
	}

	p.mu.Lock()
	p.requests = append(p.requests, req)
	p.mu.Unlock()

	res := map[string]string{"Error": "secret not found"}
	if value, ok := p.secrets[req["Resource"]+":"+req["SecretKey"]]; ok {
		res = map[string]string{"SecretValue": base64.StdEncoding.EncodeToString([]byte(value))}
	}

	out, _ := json.Marshal(res)
	msg := binary.BigEndian.AppendUint32(nil, secretAgentMagic)
	msg = binary.BigEndian.AppendUint32(msg, uint32(len(out)))
	_, _ = conn.Write(append(msg, out...))
}

func requireNoConnections(t *testing.T, agent, server *fakePeer) {
	t.Helper()

	if n := agent.Conns(); n != 0 {
		t.Errorf("expected no Secret Agent connection, got %d", n)
	}

	if n := server.Conns(); n != 0 {
		t.Errorf("expected no server connection, got %d", n)
	}
}

type serverCommand struct {
	serverErr error
	path      []string
	args      []string
}

var serverCommands = map[string]serverCommand{
	"generate": {path: []string{"generate"}, serverErr: errUnableToGenerateConfigFile},
	"diff server": {
		path:      []string{"diff", "server"},
		args:      []string{diffServerInput},
		serverErr: errUnableToGenerateConfigFromServer,
	},
}

// runServerCommand runs the command from the root command with configText as
// its config file, so no other astools.conf is read. It returns the command
// for its flags.
func runServerCommand(t *testing.T, sc serverCommand, configText string, args ...string) (*cobra.Command, error) {
	t.Helper()

	config.Reset()
	t.Cleanup(config.Reset)

	configFile := filepath.Join(t.TempDir(), "astools.conf")
	if err := os.WriteFile(configFile, []byte(configText), outputFilePermissions); err != nil {
		t.Fatal(err)
	}

	*cfFileFlags = flags.ConfFileFlags{}

	root := NewRootCmd()
	root.AddCommand(newGenerateCmd())
	root.AddCommand(newDiffCmd())

	cmdArgs := append([]string{}, sc.path...)
	cmdArgs = append(cmdArgs, "--config-file", configFile)
	cmdArgs = append(cmdArgs, args...)
	root.SetArgs(append(cmdArgs, sc.args...))

	err := root.Execute()

	cmd, _, findErr := root.Find(sc.path)
	if findErr != nil {
		t.Fatal(findErr)
	}

	return cmd, err
}

func requirePassword(t *testing.T, cmd *cobra.Command, want string) {
	t.Helper()

	if got := cmd.Flags().Lookup("password").Value.String(); got != want {
		t.Fatalf("expected the password %q, got %q", want, got)
	}
}

func requireErrText(t *testing.T, err error, contains ...string) {
	t.Helper()

	if err == nil {
		t.Fatalf("expected an error containing %q", contains)
	}

	for _, s := range contains {
		if !strings.Contains(err.Error(), s) {
			t.Fatalf("expected error to contain %q, got %v", s, err)
		}
	}
}

// captureOutput returns what run writes to standard output, standard error
// and the logger.
func captureOutput(t *testing.T, run func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	stdout, stderr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = w, w

	logger.SetOutput(w)

	out := make(chan string)

	go func() {
		data, _ := io.ReadAll(r)
		out <- string(data)
	}()

	defer func() {
		os.Stdout, os.Stderr = stdout, stderr
		logger.SetOutput(os.Stderr)
	}()

	run()

	_ = w.Close()

	return <-out
}

func TestBadSecretAgentSectionFailsClosed(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}

	sections := map[string]struct {
		section string
		want    string
	}{
		"unknown key": {
			section: `sa-adress = "127.0.0.1"`,
			want:    "unknown key sa-adress in the [secret-agent] config section",
		},
		"invalid port": {
			section: `sa-port = "0"`,
			want:    `invalid value "0" for sa-port in the [secret-agent] config section, expected an integer from 1 to 65535`,
		},
		"invalid timeout": {
			section: `sa-timeout = 0`,
			want: `invalid value "0" for sa-timeout in the [secret-agent] config section, ` +
				"expected an integer from 1 to 2147483647",
		},
		"invalid address": {
			section: `sa-address = "a:b:c"`,
			want: `invalid value "a:b:c" for sa-address in the [secret-agent] config section, ` +
				"expected <host>[:<port>] or [<ipv6>][:<port>]",
		},
		"timeout of the wrong type": {
			section: `sa-timeout = true`,
			want: `invalid value "true" for sa-timeout in the [secret-agent] config section, ` +
				"expected an integer from 1 to 2147483647",
		},
		"address of the wrong type": {
			section: `sa-address = ["127.0.0.1"]`,
			want:    "for sa-address in the [secret-agent] config section, expected <host>[:<port>] or [<ipv6>][:<port>]",
		},
		"empty CA file": {
			section: `sa-ca-file = ""`,
			want: `invalid value "" for sa-ca-file in the [secret-agent] config section, ` +
				"expected a path to a CA certificate file",
		},
		"empty legacy CA file": {
			section: `sa-cafile = ""`,
			want: `invalid value "" for sa-cafile in the [secret-agent] config section, ` +
				"expected a path to a CA certificate file",
		},
	}

	passwords := map[string]string{
		"with a secret":   "secrets:asconfig:password",
		"without secrets": "admin",
	}

	for cmdName, sc := range serverCommands {
		for name, tt := range sections {
			for passwordName, password := range passwords {
				t.Run(cmdName+"/"+name+"/"+passwordName, func(t *testing.T) {
					agent := newFakeAgent(t, map[string]string{"asconfig:password": "admin"})
					server := newFakeServer(t)

					configText := "[cluster]\nhost = \"" + server.Addr() + "\"\nuser = \"admin\"\n" +
						"password = \"" + password + "\"\n\n[secret-agent]\n" + tt.section + "\n"

					_, err := runServerCommand(t, sc, configText, "--sa-address", agent.Addr())

					requireErrText(t, err, tt.want)
					requireNoConnections(t, agent, server)
				})
			}
		}
	}
}

func TestBadSecretAgentSectionFailsClosedWithInstance(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}

	for cmdName, sc := range serverCommands {
		t.Run(cmdName, func(t *testing.T) {
			agent := newFakeAgent(t, map[string]string{"asconfig:password": "admin"})
			server := newFakeServer(t)

			configText := "[cluster_a]\nhost = \"" + server.Addr() + "\"\nuser = \"admin\"\npassword = \"admin\"\n\n" +
				"[secret-agent]\nsa-address = \"" + agent.Addr() + "\"\n\n[secret-agent_a]\nsa-port = \"0\"\n"

			_, err := runServerCommand(t, sc, configText, "--instance", "a")

			requireErrText(t, err,
				`invalid value "0" for sa-port in the [secret-agent_a] config section, expected an integer from 1 to 65535`)
			requireNoConnections(t, agent, server)
		})
	}
}

func TestMissingSecretAgentCAFileInConfigFile(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}

	for cmdName, sc := range serverCommands {
		t.Run(cmdName, func(t *testing.T) {
			agent := newFakeAgent(t, map[string]string{"asconfig:password": "admin"})
			server := newFakeServer(t)

			configText := "[cluster]\nhost = \"" + server.Addr() + "\"\nuser = \"admin\"\n" +
				"password = \"secrets:asconfig:password\"\n\n[secret-agent]\nsa-address = \"" + agent.Addr() + "\"\n" +
				"sa-ca-file = \"/nonexistent/ca.pem\"\n"

			_, err := runServerCommand(t, sc, configText)

			requireErrText(t, err, "sa-ca-file in the [secret-agent] config section", "/nonexistent/ca.pem")
			requireNoConnections(t, agent, server)
		})
	}
}

func TestEmptySecretAgentCAFileOption(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}

	for cmdName, sc := range serverCommands {
		for _, option := range []string{"sa-ca-file", "sa-cafile"} {
			t.Run(cmdName+"/"+option, func(t *testing.T) {
				agent := newFakeAgent(t, map[string]string{"asconfig:password": "admin"})
				server := newFakeServer(t)

				_, err := runServerCommand(t, sc, "",
					"-h", server.Addr(), "-U", "admin", "-P", "secrets:asconfig:password",
					"--sa-address", agent.Addr(), "--"+option, "")

				requireErrText(t, err, "--"+option+`: invalid value "", expected a path to a CA certificate file`)
				requireNoConnections(t, agent, server)
			})
		}
	}
}

func TestSecretAgentTimeoutIsDecimal(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}

	tests := map[string]struct {
		timeout string
		valid   bool
	}{
		"plus sign":               {timeout: "+5"},
		"hexadecimal":             {timeout: "0x10"},
		"leading zero is decimal": {timeout: "0900", valid: true},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			agent := newFakeAgent(t, map[string]string{"asconfig:password": "admin"})
			server := newFakeServer(t)

			cmd, err := runServerCommand(t, serverCommands["generate"], "",
				"-h", server.Addr(), "-U", "admin", "-P", "secrets:asconfig:password",
				"--sa-address", agent.Addr(), "--sa-timeout", tt.timeout)

			if tt.valid {
				requireErr(t, err, errUnableToGenerateConfigFile)
				requirePassword(t, cmd, "admin")

				return
			}

			requireErrText(t, err,
				`--sa-timeout: invalid value "`+tt.timeout+`", expected an integer from 1 to 2147483647`)
			requireNoConnections(t, agent, server)
		})
	}
}

func TestLegacyCAFileDeprecation(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}

	for cmdName, sc := range serverCommands {
		t.Run(cmdName, func(t *testing.T) {
			server := newFakeServer(t)

			output := captureOutput(t, func() {
				_, _ = runServerCommand(t, sc, "", "-h", server.Addr(), "--sa-cafile", "/nonexistent/ca.pem")
			})

			if n := strings.Count(output, legacyCADeprecation); n != 1 {
				t.Fatalf("expected %q once, got %d times in:\n%s", legacyCADeprecation, n, output)
			}

			if strings.Contains(output, "has been deprecated") {
				t.Fatalf("expected only the asadm wording, got:\n%s", output)
			}
		})
	}
}

func TestSecretAgentHelp(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}

	help := map[string][]string{
		"sa-address": {`"<host>[:<port>]" or "[<ipv6>][:<port>]"`, "Secret Agent hostname or IP address."},
		"sa-port":    {"Secret Agent port, 1 to 65535. Overrides a port in --sa-address."},
		"sa-timeout": {"Timeout in milliseconds for Secret Agent requests, an integer from 1 to 2147483647."},
		"sa-ca-file": {"Path to a CA certificate file. Enables TLS to the Secret Agent and verifies its " +
			"certificate against this CA. The agent's hostname or IP address must be in the certificate."},
	}

	defaults := map[string]string{
		"sa-address": "127.0.0.1:3005",
		"sa-port":    "3005",
		"sa-timeout": "1000",
		"sa-ca-file": "none",
	}

	commands := map[string]*cobra.Command{
		"generate":    newGenerateCmd(),
		"diff server": newDiffServerCmd(),
	}

	for cmdName, cmd := range commands {
		for name, sentences := range help {
			usage := flagHelp(t, cmd, name)

			for _, sentence := range sentences {
				if want := strings.Join(strings.Fields(sentence), " "); !strings.Contains(usage, want) {
					t.Errorf("%s: --%s help is missing %q, got %q", cmdName, name, want, usage)
				}
			}

			if def, ok := defaults[name]; ok &&
				!strings.Contains(usage, "Default: "+def) && !strings.Contains(usage, "(default "+def+")") {
				t.Errorf("%s: --%s help is missing the default %s, got %q", cmdName, name, def, usage)
			}
		}
	}
}

func TestSecretAgentHelpBlock(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}

	const tlsPath = " A --tls-cafile, --tls-certfile or --tls-keyfile path that begins with\n" +
		" 'secrets:' must be given as './secrets:...'.\n"

	const block = "\n\n[secret-agent]\n" +
		" Aerospike Secret Agent used for 'secrets:<resource>:<key>' values.\n" +
		" In a config file, set these in the [secret-agent] section as sa-address,\n" +
		" sa-port, sa-timeout and sa-ca-file.\n" + tlsPath

	commands := map[string]*cobra.Command{
		"generate":    newGenerateCmd(),
		"diff server": newDiffServerCmd(),
	}

	for cmdName, cmd := range commands {
		usage := cmd.UsageString()

		if !strings.Contains(usage, tlsPath) {
			t.Errorf("%s: the [secret-agent] block does not say how to name a TLS file that begins with "+
				"'secrets:':\n%s", cmdName, usage)
		}

		before, after, found := strings.Cut(usage, block)
		if !found {
			t.Fatalf("%s: help has no [secret-agent] block:\n%s", cmdName, usage)
		}

		if _, flagList, _ := strings.Cut(before, "\nFlags:\n"); strings.Contains(flagList, "--sa-") {
			t.Errorf("%s: --sa-* options are listed outside the [secret-agent] block:\n%s", cmdName, usage)
		}

		saFlags, _, _ := strings.Cut(after, "\n\n")
		if !strings.HasPrefix(strings.TrimSpace(saFlags), "--sa-address") {
			t.Errorf("%s: the [secret-agent] block does not start with --sa-address:\n%s", cmdName, saFlags)
		}

		for _, name := range []string{"--sa-address", "--sa-port", "--sa-timeout", "--sa-ca-file"} {
			if !strings.Contains(saFlags, name) {
				t.Errorf("%s: the [secret-agent] block is missing %s:\n%s", cmdName, name, saFlags)
			}
		}

		if strings.Contains(usage, "--sa-cafile") {
			t.Errorf("%s: help shows --sa-cafile", cmdName)
		}
	}
}

func TestLegacyCAFileHiddenFromHelp(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}

	commands := map[string]*cobra.Command{
		"generate":    newGenerateCmd(),
		"diff server": newDiffServerCmd(),
	}

	for cmdName, cmd := range commands {
		if cmd.Flags().Lookup("sa-cafile") == nil {
			t.Errorf("%s: missing --sa-cafile", cmdName)
		}

		if usage := cmd.Flags().FlagUsages(); strings.Contains(usage, "sa-cafile") {
			t.Errorf("%s: help shows --sa-cafile", cmdName)
		}
	}
}

func TestDiffServerFlagsDoNotCarryOver(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}

	sc := serverCommands["diff server"]
	first := newFakeServer(t)
	second := newFakeServer(t)

	for _, server := range []*fakePeer{first, second} {
		_, err := runServerCommand(t, sc, "", "-h", server.Addr(), "-U", "admin", "-P", "admin")
		requireErr(t, err, sc.serverErr)
	}

	if n := second.Conns(); n == 0 {
		t.Fatal("expected the second run to connect to its own -h, not to a host from an earlier run")
	}
}

func TestTLSFileSecretsNeedTLS(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}

	for _, option := range []string{"tls-cafile", "tls-certfile", "tls-keyfile"} {
		for _, tlsEnabled := range []bool{false, true} {
			name := option + "/without TLS"
			if tlsEnabled {
				name = option + "/with TLS"
			}

			t.Run(name, func(t *testing.T) {
				agent := newFakeAgent(t, map[string]string{"asconfig:pem": "not a pem"})
				server := newFakeServer(t)

				args := []string{"-h", server.Addr(), "--" + option, "secrets:asconfig:pem", "--sa-address", agent.Addr()}
				if tlsEnabled {
					args = append(args, "--tls-enable")
				}

				_, err := runServerCommand(t, serverCommands["generate"], "", args...)
				if err == nil {
					t.Fatal("expected an error")
				}

				if n := agent.Conns(); tlsEnabled && n == 0 {
					t.Fatalf("expected --%s to be fetched with TLS enabled", option)
				} else if !tlsEnabled && n != 0 {
					t.Fatalf("expected --%s not to be fetched without TLS, got %d agent connections", option, n)
				}
			})
		}
	}
}

func TestSecretAgentRequest(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}

	tests := map[string]struct {
		request map[string]string
		ref     string
	}{
		"key only, as in the C tools and asadm": {
			ref:     "secrets:asconfig",
			request: map[string]string{"SecretKey": "asconfig"},
		},
		"resource with colons": {
			ref:     "secrets:a:b:password",
			request: map[string]string{"Resource": "a:b", "SecretKey": "password"},
		},
		"quotes and backslashes": {
			ref:     `secrets:a"b\c:d"e\f`,
			request: map[string]string{"Resource": `a"b\c`, "SecretKey": `d"e\f`},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			agent := newFakeAgent(t, map[string]string{
				tt.request["Resource"] + ":" + tt.request["SecretKey"]: "admin",
			})
			server := newFakeServer(t)

			cmd, err := runServerCommand(t, serverCommands["generate"], "",
				"-h", server.Addr(), "-U", "admin", "-P", tt.ref, "--sa-address", agent.Addr())

			requireErr(t, err, errUnableToGenerateConfigFile)
			requirePassword(t, cmd, "admin")

			if got := agent.Requests(); !reflect.DeepEqual(got, []map[string]string{tt.request}) {
				t.Fatalf("expected the request %v, got %v", tt.request, got)
			}
		})
	}
}

func TestSecretAgentCAFile(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}

	agent := secretagenttest.NewTLSServer(t, secretagenttest.Secrets{
		"asconfig": {"password": base64.StdEncoding.EncodeToString([]byte("admin"))},
	})
	otherCA := secretagenttest.NewTLSServer(t, nil).CAFile()

	tests := map[string]struct {
		args     []string
		contains []string
		fetched  bool
	}{
		"--sa-ca-file beats a later --sa-cafile": {
			args:    []string{"--sa-ca-file", agent.CAFile(), "--sa-cafile", otherCA},
			fetched: true,
		},
		"--sa-ca-file beats an earlier --sa-cafile": {
			args:    []string{"--sa-cafile", otherCA, "--sa-ca-file", agent.CAFile()},
			fetched: true,
		},
		"--sa-cafile alone": {
			args:    []string{"--sa-cafile", agent.CAFile()},
			fetched: true,
		},
		"--sa-ca-file wins over a valid --sa-cafile": {
			args:     []string{"--sa-cafile", agent.CAFile(), "--sa-ca-file", otherCA},
			contains: []string{"--password", "secrets:asconfig:password"},
		},
		"untrusted CA": {
			args:     []string{"--sa-ca-file", otherCA},
			contains: []string{"--password", "secrets:asconfig:password"},
		},
		"no CA for a TLS agent": {
			contains: []string{"--password", "secrets:asconfig:password"},
		},
		"missing CA file": {
			args:     []string{"--sa-ca-file", "/nonexistent/ca.pem"},
			contains: []string{"--sa-ca-file", "/nonexistent/ca.pem"},
		},
	}

	for cmdName, sc := range serverCommands {
		for name, tt := range tests {
			t.Run(cmdName+"/"+name, func(t *testing.T) {
				server := newFakeServer(t)

				args := append([]string{
					"-h", server.Addr(), "-U", "admin", "-P", "secrets:asconfig:password",
					"--sa-address", agent.Addr(), "--sa-timeout", "5000",
				}, tt.args...)

				cmd, err := runServerCommand(t, sc, "", args...)

				if tt.fetched {
					requireErr(t, err, sc.serverErr)
					requirePassword(t, cmd, "admin")

					return
				}

				requireErr(t, err, errUnableToResolveSecrets, tt.contains...)

				if n := server.Conns(); n != 0 {
					t.Errorf("expected no server connection, got %d", n)
				}
			})
		}
	}
}

func TestSecretAgentSettingsMergeConfigAndCommandLine(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}

	agent := newFakeAgent(t, map[string]string{"asconfig:password": "admin"})
	tlsAgent := secretagenttest.NewTLSServer(t, secretagenttest.Secrets{
		"asconfig": {"password": base64.StdEncoding.EncodeToString([]byte("admin"))},
	})

	tests := map[string]struct {
		section string
		args    []string
	}{
		"command line port beats a port in the config file address": {
			section: "sa-address = \"127.0.0.1:1\"\n",
			args:    []string{"--sa-port", agent.Port()},
		},
		"config file CA with a command line address": {
			section: "sa-ca-file = \"" + tlsAgent.CAFile() + "\"\n",
			args:    []string{"--sa-address", tlsAgent.Addr()},
		},
	}

	for cmdName, sc := range serverCommands {
		for name, tt := range tests {
			t.Run(cmdName+"/"+name, func(t *testing.T) {
				server := newFakeServer(t)

				configText := "[cluster]\nhost = \"" + server.Addr() + "\"\nuser = \"admin\"\n" +
					"password = \"secrets:asconfig:password\"\n\n[secret-agent]\n" + tt.section

				cmd, err := runServerCommand(t, sc, configText, tt.args...)

				requireErr(t, err, sc.serverErr)
				requirePassword(t, cmd, "admin")
			})
		}
	}
}

func TestCommandLineBeatsConfigFileSecret(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}

	for cmdName, sc := range serverCommands {
		t.Run(cmdName, func(t *testing.T) {
			agent := newFakeAgent(t, nil)
			server := newFakeServer(t)

			configText := "[cluster]\nhost = \"" + server.Addr() + "\"\nuser = \"secrets:asconfig:user\"\n" +
				"password = \"secrets:asconfig:password\"\n\n[secret-agent]\nsa-address = \"" + agent.Addr() + "\"\n"

			cmd, err := runServerCommand(t, sc, configText, "-U", "admin", "-P", "admin")

			requireErr(t, err, sc.serverErr)
			requirePassword(t, cmd, "admin")

			if n := agent.Conns(); n != 0 {
				t.Fatalf("expected no Secret Agent connection, got %d", n)
			}
		})
	}
}

func TestServerCommandsResolveSecrets(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}

	for cmdName, sc := range serverCommands {
		t.Run(cmdName+"/fetched", func(t *testing.T) {
			agent := newFakeAgent(t, map[string]string{"asconfig:password": "admin"})
			server := newFakeServer(t)

			cmd, err := runServerCommand(t, sc, "",
				"-h", server.Addr(), "-U", "admin", "-P", "secrets:asconfig:password", "--sa-address", agent.Addr())

			requireErr(t, err, sc.serverErr)
			requirePassword(t, cmd, "admin")

			if n := server.Conns(); n == 0 {
				t.Fatal("expected a server connection")
			}
		})

		t.Run(cmdName+"/missing key", func(t *testing.T) {
			agent := newFakeAgent(t, nil)
			server := newFakeServer(t)

			_, err := runServerCommand(t, sc, "",
				"-h", server.Addr(), "-U", "admin", "-P", "secrets:asconfig:missing", "--sa-address", agent.Addr())

			requireErr(t, err, errUnableToResolveSecrets, "--password", "secrets:asconfig:missing")

			if n := server.Conns(); n != 0 {
				t.Fatalf("expected no server connection, got %d", n)
			}
		})
	}
}

func TestGenerateOpensOutputAfterCredentials(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}

	agent := newFakeAgent(t, nil)
	server := newFakeServer(t)
	output := filepath.Join(t.TempDir(), "out.conf")

	_, err := runServerCommand(t, serverCommands["generate"], "",
		"-h", server.Addr(), "-U", "admin", "-P", "secrets:asconfig:missing", "--sa-address", agent.Addr(),
		"-o", output)

	requireErr(t, err, errUnableToResolveSecrets)

	if _, statErr := os.Stat(output); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected %s not to be created, got %v", output, statErr)
	}
}

func requireResolveErr(t *testing.T, err error, want string) {
	t.Helper()

	requireErr(t, err, errUnableToResolveSecrets)

	if got := err.Error(); got != errUnableToResolveSecrets.Error()+": "+want {
		t.Fatalf("expected the error %q, got %q", want, got)
	}
}

func TestSecretAgentOptionsCheckedWithoutSecrets(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}

	tests := map[string]struct {
		want string
		args []string
	}{
		"address": {
			args: []string{"--sa-address", "[localhost]:3005"},
			want: `--sa-address: invalid value "[localhost]:3005", expected <host>[:<port>] or [<ipv6>][:<port>]`,
		},
		"port": {
			args: []string{"--sa-port", "65536"},
			want: `--sa-port: invalid value "65536", expected an integer from 1 to 65535`,
		},
		"timeout": {
			args: []string{"--sa-timeout", "0"},
			want: `--sa-timeout: invalid value "0", expected an integer from 1 to 2147483647`,
		},
		"CA file": {
			args: []string{"--sa-ca-file", ""},
			want: `--sa-ca-file: invalid value "", expected a path to a CA certificate file`,
		},
		"legacy CA file": {
			args: []string{"--sa-cafile", ""},
			want: `--sa-cafile: invalid value "", expected a path to a CA certificate file`,
		},
	}

	for cmdName, sc := range serverCommands {
		for name, tt := range tests {
			t.Run(cmdName+"/"+name, func(t *testing.T) {
				server := newFakeServer(t)

				_, err := runServerCommand(t, sc, "",
					append([]string{"-h", server.Addr(), "-U", "admin", "-P", "admin"}, tt.args...)...)

				requireResolveErr(t, err, tt.want)

				if n := server.Conns(); n != 0 {
					t.Errorf("expected no server connection, got %d", n)
				}
			})
		}
	}
}

func TestSecretAgentCAFileReadOnlyWhenFetching(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}

	const missing = "/nonexistent/ca.pem"

	tlsAgent := secretagenttest.NewTLSServer(t, secretagenttest.Secrets{
		"asconfig": {"password": base64.StdEncoding.EncodeToString([]byte("admin"))},
	})

	for cmdName, sc := range serverCommands {
		for _, key := range []string{"sa-ca-file", "sa-cafile"} {
			t.Run(cmdName+"/config "+key+" not read without a secret", func(t *testing.T) {
				server := newFakeServer(t)

				configText := "[cluster]\nhost = \"" + server.Addr() + "\"\nuser = \"admin\"\npassword = \"admin\"\n\n" +
					"[secret-agent]\n" + key + " = \"" + missing + "\"\n"

				_, err := runServerCommand(t, sc, configText)

				requireErr(t, err, sc.serverErr)
			})
		}

		for _, option := range []string{"--sa-ca-file", "--sa-cafile"} {
			t.Run(cmdName+"/"+option+" beats a config file CA file that cannot be read", func(t *testing.T) {
				server := newFakeServer(t)

				configText := "[cluster]\nhost = \"" + server.Addr() + "\"\nuser = \"admin\"\n" +
					"password = \"secrets:asconfig:password\"\n\n[secret-agent]\nsa-address = \"" + tlsAgent.Addr() + "\"\n" +
					"sa-ca-file = \"" + missing + "\"\n"

				cmd, err := runServerCommand(t, sc, configText, option, tlsAgent.CAFile())

				requireErr(t, err, sc.serverErr)
				requirePassword(t, cmd, "admin")
			})
		}
	}

	tests := map[string]struct {
		conf     string
		instance string
		want     string
		args     []string
	}{
		"--sa-ca-file": {
			args: []string{"--sa-ca-file", missing},
			want: `--sa-ca-file: cannot load CA file "` + missing + `": No such file or directory`,
		},
		"--sa-cafile": {
			args: []string{"--sa-cafile", missing},
			want: `--sa-cafile: cannot load CA file "` + missing + `": No such file or directory`,
		},
		"config sa-ca-file": {
			conf: "[secret-agent]\nsa-ca-file = \"" + missing + "\"\n",
			want: `sa-ca-file in the [secret-agent] config section: cannot load CA file "` + missing + `": No such file or directory`,
		},
		"config sa-cafile": {
			conf: "[secret-agent]\nsa-cafile = \"" + missing + "\"\n",
			want: `sa-cafile in the [secret-agent] config section: cannot load CA file "` + missing + `": No such file or directory`,
		},
		"instance section": {
			conf:     "[secret-agent_a]\nsa-ca-file = \"" + missing + "\"\n",
			instance: "a",
			want:     `sa-ca-file in the [secret-agent_a] config section: cannot load CA file "` + missing + `": No such file or directory`,
		},
	}

	for cmdName, sc := range serverCommands {
		for name, tt := range tests {
			t.Run(cmdName+"/"+name, func(t *testing.T) {
				agent := newFakeAgent(t, map[string]string{"asconfig:password": "admin"})
				server := newFakeServer(t)

				args := append([]string{
					"-h", server.Addr(), "-U", "admin", "-P", "secrets:asconfig:password", "--sa-address", agent.Addr(),
				}, tt.args...)

				if tt.instance != "" {
					args = append(args, "--instance", tt.instance)
				}

				_, err := runServerCommand(t, sc, tt.conf, args...)

				requireResolveErr(t, err, tt.want)
				requireNoConnections(t, agent, server)
			})
		}
	}
}

func TestSecretAgentOnlySelectedSectionIsRead(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}

	const invalid = "sa-port = 0\nsa-ca-file = \"\"\nbogus = 1\n"

	tests := map[string]struct {
		conf     string
		instance string
		args     []string
	}{
		"instance section, invalid default section": {
			conf:     "[secret-agent]\n" + invalid + "\n[secret-agent_a]\nsa-address = \"AGENT\"\n",
			instance: "a",
		},
		"empty instance section, invalid default section": {
			conf:     "[secret-agent]\n" + invalid + "\n[secret-agent_a]\n",
			instance: "a",
			args:     []string{"--sa-address", "AGENT"},
		},
		"no instance, invalid instance section": {
			conf: "[secret-agent]\nsa-address = \"AGENT\"\n\n[secret-agent_a]\n" + invalid,
		},
		"other instance, invalid instance section": {
			conf:     "[secret-agent]\nsa-address = \"AGENT\"\n\n[secret-agent_a]\n" + invalid,
			instance: "b",
		},
	}

	for cmdName, sc := range serverCommands {
		for name, tt := range tests {
			t.Run(cmdName+"/"+name, func(t *testing.T) {
				agent := newFakeAgent(t, map[string]string{"asconfig:password": "admin"})
				server := newFakeServer(t)

				args := []string{"-h", server.Addr(), "-U", "admin", "-P", "secrets:asconfig:password"}
				for _, arg := range tt.args {
					args = append(args, strings.ReplaceAll(arg, "AGENT", agent.Addr()))
				}

				if tt.instance != "" {
					args = append(args, "--instance", tt.instance)
				}

				cmd, err := runServerCommand(t, sc, strings.ReplaceAll(tt.conf, "AGENT", agent.Addr()), args...)

				requireErr(t, err, sc.serverErr)
				requirePassword(t, cmd, "admin")
			})
		}
	}
}

func TestFetchedUserIsNotPrinted(t *testing.T) {
	if err := InitializeGlobals(); err != nil {
		t.Fatalf("Failed to initialize globals for testing: %v", err)
	}

	const (
		fetchedUser     = "UserUniq-4412"
		fetchedPassword = "PassUniq-9931"
		literalUser     = "LiteralUser-7781"
	)

	tests := map[string]struct {
		config  string
		printed string
		args    []string
	}{
		"user from the command line": {
			args:    []string{"-U", "secrets:asconfig:user", "-P", "secrets:asconfig:password"},
			printed: "secrets:asconfig:user",
		},
		"user from the config file": {
			config:  "[cluster]\nuser = \"secrets:asconfig:user\"\npassword = \"secrets:asconfig:password\"\n",
			printed: "secrets:asconfig:user",
		},
		"literal user": {
			args:    []string{"-U", literalUser, "-P", "secrets:asconfig:password"},
			printed: literalUser,
		},
	}

	for cmdName, sc := range serverCommands {
		for name, tt := range tests {
			for _, level := range []string{"info", "trace"} {
				t.Run(cmdName+"/"+name+"/"+level, func(t *testing.T) {
					agent := newFakeAgent(t, map[string]string{
						"asconfig:user":     fetchedUser,
						"asconfig:password": fetchedPassword,
					})
					server := newFakeServer(t)

					args := append([]string{"-h", server.Addr(), "--sa-address", agent.Addr(), "--log-level", level},
						tt.args...)

					var err error

					output := captureOutput(t, func() { _, err = runServerCommand(t, sc, tt.config, args...) })

					requireErr(t, err, sc.serverErr)
					requireLoginFailurePrints(t, output, err, tt.printed, fetchedUser, fetchedPassword)
				})
			}
		}
	}
}

// requireLoginFailurePrints checks that the log and err name user in the
// failed login and print none of hidden.
func requireLoginFailurePrints(t *testing.T, output string, err error, user string, hidden ...string) {
	t.Helper()

	printed := output + "\n" + err.Error()

	for _, value := range hidden {
		if strings.Contains(printed, value) {
			t.Fatalf("expected the fetched value %q not to be printed, got:\n%s", value, printed)
		}
	}

	want := "failed to authenticate user `" + user + "`"
	if !strings.Contains(err.Error(), want) || !strings.Contains(output, want) {
		t.Fatalf("expected %q in the error and the log, got:\n%s", want, printed)
	}
}
