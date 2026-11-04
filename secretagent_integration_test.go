//go:build integration

package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aerospike/tools-common-go/secretagent"
	"github.com/aerospike/tools-common-go/secretagent/secretagenttest"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

const (
	secretAgentImage         = "aerospike/aerospike-secret-agent:1.1.0"
	secretAgentPlatform      = "linux/amd64"
	secretAgentServerVersion = "ee-6.4.0.2"
	secretAgentPassword      = "admin"
	secretAgentWrongPassword = "not-the-admin-password"
	secretAgentUnknownUser   = "UserUniq-4412"
	secretAgentStartTimeout  = 2 * time.Minute
	legacyCADeprecation      = "Argument --sa-cafile is deprecated. Use --sa-ca-file instead."
	passwordEnv              = "ASCONFIG_E2E_PASSWORD"
	keyPasswordEnv           = "ASCONFIG_E2E_KEY_PASSWORD"
	keyPassword              = "k3y-pass"
	tlsServerPort            = "4333/tcp"
)

// containerName returns a unique container name. ASCONFIG_TEST_CONTAINER_PREFIX
// sets its prefix.
func containerName(role string) string {
	prefix := os.Getenv("ASCONFIG_TEST_CONTAINER_PREFIX")
	if prefix == "" {
		prefix = "asconfig-test-"
	}

	suffix := make([]byte, 4)
	_, _ = rand.Read(suffix)

	return prefix + role + "-" + hex.EncodeToString(suffix)
}

func ensureImage(t *testing.T, dockerClient *client.Client, ref string, options image.PullOptions) {
	t.Helper()

	ctx := context.Background()

	if _, err := dockerClient.ImageInspect(ctx, ref); err == nil {
		return
	}

	rc, err := dockerClient.ImagePull(ctx, ref, options)
	if err != nil {
		t.Fatalf("pull %s: %v", ref, err)
	}

	_, err = io.Copy(io.Discard, rc)
	rc.Close()

	if err != nil {
		t.Fatalf("pull %s: %v", ref, err)
	}
}

// startContainer starts a container whose port is published on a free port of
// 127.0.0.1, which works on Linux and on Docker Desktop, where container
// addresses are not reachable from the host. It returns 127.0.0.1:<port>.
func startContainer(
	t *testing.T, dockerClient *client.Client, role string, cfg *container.Config, hostCfg *container.HostConfig,
	port nat.Port,
) string {
	t.Helper()

	ctx := context.Background()

	cfg.ExposedPorts = nat.PortSet{port: {}}
	hostCfg.PortBindings = nat.PortMap{port: {{HostIP: "127.0.0.1"}}}

	created, err := dockerClient.ContainerCreate(ctx, cfg, hostCfg, nil, nil, containerName(role))
	if err != nil {
		t.Fatalf("create %s container: %v", role, err)
	}

	t.Cleanup(func() {
		_ = dockerClient.ContainerRemove(context.Background(), created.ID, container.RemoveOptions{Force: true})
	})

	if err = dockerClient.ContainerStart(ctx, created.ID, container.StartOptions{}); err != nil {
		t.Fatalf("start %s container: %v", role, err)
	}

	info, err := dockerClient.ContainerInspect(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}

	bindings := info.NetworkSettings.Ports[port]
	if len(bindings) == 0 {
		t.Fatalf("%s port %s is not published", role, port)
	}

	return net.JoinHostPort("127.0.0.1", bindings[0].HostPort)
}

// startSecretAgent runs a Secret Agent with a file backend holding the server
// password and extra, and returns its address once it serves secrets. With pki
// set it serves TLS with the PKI's server certificate.
func startSecretAgent(
	t *testing.T, dockerClient *client.Client, pki *secretagenttest.PKI, extra map[string]string,
) string {
	t.Helper()

	ensureImage(t, dockerClient, secretAgentImage, image.PullOptions{Platform: secretAgentPlatform})

	dir := t.TempDir()

	values := map[string]string{
		"password": secretAgentPassword,
		"wrong":    secretAgentWrongPassword,
	}
	maps.Copy(values, extra)

	secrets, err := json.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}

	files := map[string][]byte{"asconfig.json": secrets}
	serviceTLS := ""
	role := "secret-agent"
	clientTLS := secretagent.TLSOptions{}

	if pki != nil {
		for _, path := range []string{pki.ServerCertFile, pki.ServerKeyFile} {
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatal(readErr)
			}

			files[filepath.Base(path)] = data
		}

		serviceTLS = fmt.Sprintf("    tls:\n      cert-file: /agent/%s\n      key-file: /agent/%s\n",
			filepath.Base(pki.ServerCertFile), filepath.Base(pki.ServerKeyFile))
		role = "secret-agent-tls"
		clientTLS.CAFile = pki.CAFile
	}

	files["agent.yaml"] = []byte("service:\n  tcp:\n    endpoint: 0.0.0.0:3005\n" + serviceTLS +
		"secret-manager:\n  file:\n    convert-to-base64: true\n    resources:\n" +
		"      asconfig: \"/agent/asconfig.json\"\n" +
		"log:\n  level: debug\n")

	// World-readable so the agent can read them whatever uid Docker maps it to.
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), data, tempFilePermissions); err != nil {
			t.Fatal(err)
		}
	}

	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	addr := startContainer(t, dockerClient, role,
		&container.Config{Image: secretAgentImage, Cmd: []string{"--config-file", "/agent/agent.yaml"}},
		&container.HostConfig{
			Mounts: []mount.Mount{{Type: mount.TypeBind, Source: dir, Target: "/agent", ReadOnly: true}},
		},
		"3005/tcp")

	tlsConfig, err := secretagent.NewTLSConfig(clientTLS)
	if err != nil {
		t.Fatal(err)
	}

	// The agent logs that it listens before its file backend has loaded, so
	// wait until it actually serves the secret.
	agentClient, err := secretagent.NewClient(secretagent.Config{
		Address: addr, TLS: tlsConfig, Base64: true, Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(secretAgentStartTimeout)

	for {
		value, resolveErr := agentClient.Resolve(context.Background(), "secrets:asconfig:password")
		if resolveErr == nil && value == secretAgentPassword {
			return addr
		}

		if time.Now().After(deadline) {
			t.Fatalf("secret agent at %s never served secrets: %v", addr, resolveErr)
		}

		time.Sleep(250 * time.Millisecond)
	}
}

// startSecretAgentServer runs an Enterprise server with security enabled and
// returns the address of port.
func startSecretAgentServer(t *testing.T, dockerClient *client.Client, confPath string, port nat.Port) string {
	t.Helper()

	serverImage := "aerospike:" + secretAgentServerVersion
	ensureImage(t, dockerClient, serverImage, image.PullOptions{})

	featureKeyPath := filepath.Join(featKeyDir, "featuresv2.conf")
	if _, err := os.Stat(featureKeyPath); err != nil {
		t.Fatalf("feature key file: %v", err)
	}

	confDir, err := filepath.Abs(filepath.Dir(confPath))
	if err != nil {
		t.Fatal(err)
	}

	return startContainer(t, dockerClient, "server",
		&container.Config{
			Image: serverImage,
			Tty:   true,
			Cmd: []string{
				"/usr/bin/asd", "--foreground", "--config-file", "/opt/aerospike/work/" + filepath.Base(confPath),
			},
		},
		&container.HostConfig{
			Mounts: []mount.Mount{
				{Type: mount.TypeBind, Source: confDir, Target: "/opt/aerospike/work", ReadOnly: true},
				{Type: mount.TypeBind, Source: featureKeyPath, Target: "/etc/aerospike/secret/features.conf"},
			},
		},
		port)
}

func runAsconfig(args ...string) (string, error) {
	return runAsconfigEnv(nil, args...)
}

// runAsconfigEnv runs asconfig with env as its only environment, besides
// GOCOVERDIR. Without HOME in env it reads no ~/.aerospike/astools.conf.
func runAsconfigEnv(env []string, args ...string) (string, error) {
	cmd := exec.Command(binPath+"/asconfig.test", args...)
	cmd.Env = append([]string{"GOCOVERDIR=" + coveragePath}, env...)

	var output bytes.Buffer

	cmd.Stdout = &output
	cmd.Stderr = &output

	err := cmd.Run()

	return output.String(), err
}

// waitForServer retries generate with a literal password, and args, until the
// server accepts it.
func waitForServer(t *testing.T, host, dir string, args ...string) {
	t.Helper()

	out := filepath.Join(dir, "baseline.conf")
	deadline := time.Now().Add(time.Minute)
	args = append([]string{"generate", "-h", host, "-U", "admin", "-P", secretAgentPassword, "-o", out}, args...)

	for {
		output, err := runAsconfig(args...)
		if err == nil {
			return
		}

		if time.Now().After(deadline) {
			t.Fatalf("server at %s never became ready: %v\n%s", host, err, output)
		}

		time.Sleep(time.Second)
	}
}

// secretAgentCase is one asconfig run of a Secret Agent test. home is written
// to ~/.aerospike/astools.conf in a new HOME, and env is added to the
// environment.
type secretAgentCase struct {
	name        string
	output      string
	home        string
	args        []string
	env         []string
	contains    []string
	notContains []string
	fail        bool
}

func runSecretAgentCases(t *testing.T, tests []secretAgentCase) {
	t.Helper()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			args := tc.args
			if tc.output != "" {
				args = append(args, "-o", tc.output)
			}

			env := tc.env

			if tc.home != "" {
				home := t.TempDir()
				if err := os.Mkdir(filepath.Join(home, ".aerospike"), 0o755); err != nil {
					t.Fatal(err)
				}

				if err := os.WriteFile(filepath.Join(home, ".aerospike", "astools.conf"), []byte(tc.home),
					tempFilePermissions); err != nil {
					t.Fatal(err)
				}

				env = append(env, "HOME="+home)
			}

			output, err := runAsconfigEnv(env, args...)

			if tc.fail {
				var exitErr *exec.ExitError
				if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
					t.Fatalf("expected asconfig to exit with 1, got %v:\n%s", err, output)
				}
			}

			if !tc.fail && err != nil {
				t.Fatalf("asconfig failed: %v\n%s", err, output)
			}

			for _, want := range tc.contains {
				if !strings.Contains(output, want) {
					t.Errorf("output does not contain %q:\n%s", want, output)
				}
			}

			for _, unwanted := range tc.notContains {
				if strings.Contains(output, unwanted) {
					t.Errorf("output contains %q:\n%s", unwanted, output)
				}
			}

			if strings.Contains(output, secretAgentWrongPassword) {
				t.Errorf("output contains a secret value:\n%s", output)
			}

			if tc.output == "" {
				return
			}

			data, readErr := os.ReadFile(tc.output)

			if tc.fail {
				if !errors.Is(readErr, os.ErrNotExist) {
					t.Errorf("expected no output file after a failure, got %v", readErr)
				}

				return
			}

			if readErr != nil || !strings.Contains(string(data), "namespace test") {
				t.Errorf("unexpected generated config (%v):\n%s", readErr, data)
			}
		})
	}
}

func TestSecretAgent(t *testing.T) {
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Fatal(err)
	}

	pki, err := secretagenttest.GeneratePKI(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	untrusted, err := secretagenttest.GeneratePKI(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	agent := startSecretAgent(t, dockerClient, nil, map[string]string{
		"user":         "admin",
		"unknown-user": secretAgentUnknownUser,
		"b64-literal":  "b64:" + base64.StdEncoding.EncodeToString([]byte(secretAgentPassword)),
		"env-literal":  "env:" + passwordEnv,
	})
	agentHost, agentPort, _ := net.SplitHostPort(agent)
	tlsAgent := startSecretAgent(t, dockerClient, &pki, nil)

	host := startSecretAgentServer(t, dockerClient, filepath.Join(expectedPath, "dim_nostorage_cluster_cr.conf"),
		"3000/tcp")

	dir := t.TempDir()
	waitForServer(t, host, dir)

	writeConf := func(name, text string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(text), tempFilePermissions); err != nil {
			t.Fatal(err)
		}

		return path
	}

	clusterConf := writeConf("astools.conf", fmt.Sprintf(`[cluster]
host = "%s"
user = "admin"
password = "secrets:asconfig:password"

[secret-agent]
sa-address = "%s"
`, host, agent))

	instanceConf := writeConf("astools-instance.conf", fmt.Sprintf(`[cluster_prod]
host = "%s"
user = "admin"
password = "secrets:asconfig:password"

[secret-agent]
sa-address = "%s"
`, host, agent))

	tlsConf := writeConf("astools-tls.conf", fmt.Sprintf(`[cluster]
host = "%s"
user = "admin"
password = "secrets:asconfig:password"

[secret-agent]
sa-address = "%s"
sa-ca-file = "%s"
`, host, tlsAgent, pki.CAFile))

	badSectionConf := writeConf("astools-bad.conf", fmt.Sprintf(`[cluster]
host = "%s"
user = "admin"
password = "%s"

[secret-agent]
sa-address = "%s"
sa-port = "0"
`, host, secretAgentPassword, agent))

	includeConf := writeConf("astools-include.conf", fmt.Sprintf("[include]\nfile = %q\n", clusterConf))
	passwordFile := writeConf("password.txt", secretAgentPassword+"\n")

	generated := filepath.Join(dir, "generated.conf")
	withPassword := func(ref string, args ...string) []string {
		return append([]string{"generate", "-h", host, "-U", "admin", "-P", ref}, args...)
	}

	passwordEnvs := []string{
		passwordEnv + "=" + secretAgentPassword,
		passwordEnv + "_B64=" + base64.StdEncoding.EncodeToString([]byte(secretAgentPassword)),
	}
	asLiteral := []string{"INVALID_CREDENTIAL"}
	clusterConfText, err := os.ReadFile(clusterConf)
	if err != nil {
		t.Fatal(err)
	}

	badSectionText, err := os.ReadFile(badSectionConf)
	if err != nil {
		t.Fatal(err)
	}

	runSecretAgentCases(t, []secretAgentCase{
		{
			name:   "--user from the Secret Agent",
			output: filepath.Join(dir, "user.conf"),
			args: []string{
				"generate", "-h", host, "-U", "secrets:asconfig:user", "-P", "secrets:asconfig:password",
				"--sa-address", agent,
			},
		},
		{
			name:     "a fetched b64: value is not decoded again",
			output:   filepath.Join(dir, "fetched-b64.conf"),
			args:     withPassword("secrets:asconfig:b64-literal", "--sa-address", agent),
			contains: asLiteral,
			fail:     true,
		},
		{
			name:     "a fetched env: value is not read again",
			output:   filepath.Join(dir, "fetched-env.conf"),
			args:     withPassword("secrets:asconfig:env-literal", "--sa-address", agent),
			env:      passwordEnvs,
			contains: asLiteral,
			fail:     true,
		},
		{
			name:   "a b64: password that decodes to secrets: is not fetched",
			output: filepath.Join(dir, "b64-secret.conf"),
			args: withPassword("b64:"+base64.StdEncoding.EncodeToString([]byte("secrets:asconfig:password")),
				"--sa-address", agent),
			contains: asLiteral,
			fail:     true,
		},
		{
			name:   "env: password",
			output: filepath.Join(dir, "env.conf"),
			args:   withPassword("env:" + passwordEnv),
			env:    passwordEnvs,
		},
		{
			name:   "env-b64: password",
			output: filepath.Join(dir, "env-b64.conf"),
			args:   withPassword("env-b64:" + passwordEnv + "_B64"),
			env:    passwordEnvs,
		},
		{
			name:   "b64: password",
			output: filepath.Join(dir, "b64.conf"),
			args:   withPassword("b64:" + base64.StdEncoding.EncodeToString([]byte(secretAgentPassword))),
		},
		{
			name:   "file: password",
			output: filepath.Join(dir, "file.conf"),
			args:   withPassword("file:" + passwordFile),
		},
		{
			name:   "~/.aerospike/astools.conf is read",
			output: filepath.Join(dir, "home.conf"),
			home:   string(clusterConfText),
			args:   []string{"generate"},
		},
		{
			name:     "a bad ~/.aerospike/astools.conf fails the run",
			output:   filepath.Join(dir, "bad-home.conf"),
			home:     string(badSectionText),
			args:     withPassword(secretAgentPassword),
			contains: []string{`invalid value \"0\" for sa-port in the [secret-agent] config section`},
			fail:     true,
		},
		{
			name:   "--no-config-file skips ~/.aerospike/astools.conf",
			output: filepath.Join(dir, "no-config-file-home.conf"),
			home:   string(badSectionText),
			args:   withPassword(secretAgentPassword, "--no-config-file"),
		},
		{
			name:   "--only-config-file skips ~/.aerospike/astools.conf",
			output: filepath.Join(dir, "only-config-file-home.conf"),
			home:   string(badSectionText),
			args:   []string{"generate", "--only-config-file", clusterConf},
		},
		{
			name:   "[include] is followed",
			output: filepath.Join(dir, "include.conf"),
			args:   []string{"generate", "--config-file", includeConf},
		},
		{
			name:   "generate, password before --sa-address",
			output: generated,
			args:   withPassword("secrets:asconfig:password", "--sa-address", agent),
		},
		{
			name:   "generate, --sa-address before password",
			output: filepath.Join(dir, "order.conf"),
			args:   []string{"generate", "--sa-address", agent, "-h", host, "-U", "admin", "-P", "secrets:asconfig:password"},
		},
		{
			name:   "generate, --sa-address and --sa-port",
			output: filepath.Join(dir, "port.conf"),
			args:   withPassword("secrets:asconfig:password", "--sa-address", agentHost, "--sa-port", agentPort),
		},
		{
			name:   "generate, config file",
			output: filepath.Join(dir, "config-file.conf"),
			args:   []string{"generate", "--config-file", clusterConf},
		},
		{
			name:   "generate, config file instance falls back to [secret-agent]",
			output: filepath.Join(dir, "instance.conf"),
			args:   []string{"generate", "--config-file", instanceConf, "--instance", "prod"},
		},
		{
			name: "diff server",
			args: []string{
				"diff", "server", "-h", host, "-U", "admin", "-P", "secrets:asconfig:password",
				"--sa-address", agent, generated,
			},
		},
		{
			name:     "missing secret",
			output:   filepath.Join(dir, "missing.conf"),
			args:     withPassword("secrets:asconfig:missing", "--sa-address", agent),
			contains: []string{"--password", "secrets:asconfig:missing"},
			fail:     true,
		},
		{
			name:     "key-only reference is sent without a resource, which agent 1.1.0 does not find",
			output:   filepath.Join(dir, "key-only.conf"),
			args:     withPassword("secrets:password", "--sa-address", agent),
			contains: []string{"--password", "secrets:password"},
			fail:     true,
		},
		{
			name:     "agent unreachable",
			output:   filepath.Join(dir, "unreachable.conf"),
			args:     withPassword("secrets:asconfig:password", "--sa-address", "127.0.0.1:1"),
			contains: []string{"--password", "secrets:asconfig:password"},
			fail:     true,
		},
		{
			name:   "fetched value is used as the password",
			output: filepath.Join(dir, "fetched.conf"),
			args:   withPassword("secrets:asconfig:password", "--sa-address", agent),
		},
		{
			name:        "a wrong fetched password is rejected by the server",
			output:      filepath.Join(dir, "wrong.conf"),
			args:        withPassword("secrets:asconfig:wrong", "--sa-address", agent),
			contains:    []string{"INVALID_CREDENTIAL"},
			notContains: []string{"unable to read credentials"},
			fail:        true,
		},
		{
			name:   "a fetched user that fails to log in is not printed",
			output: filepath.Join(dir, "unknown-user.conf"),
			args: []string{
				"generate", "-h", host, "-U", "secrets:asconfig:unknown-user", "-P", "secrets:asconfig:wrong",
				"--sa-address", agent, "--log-level", "trace",
			},
			contains:    []string{"failed to authenticate user `secrets:asconfig:unknown-user`"},
			notContains: []string{secretAgentUnknownUser, "unable to read credentials"},
			fail:        true,
		},
		{
			name: "diff server, a fetched user that fails to log in is not printed",
			args: []string{
				"diff", "server", "-h", host, "-U", "secrets:asconfig:unknown-user", "-P", "secrets:asconfig:wrong",
				"--sa-address", agent, "--log-level", "trace", generated,
			},
			contains:    []string{"failed to authenticate user `secrets:asconfig:unknown-user`"},
			notContains: []string{secretAgentUnknownUser, "unable to read credentials"},
			fail:        true,
		},
		{
			name:   "TLS agent, --sa-ca-file",
			output: filepath.Join(dir, "tls.conf"),
			args:   withPassword("secrets:asconfig:password", "--sa-address", tlsAgent, "--sa-ca-file", pki.CAFile),
		},
		{
			name:     "TLS agent, deprecated --sa-cafile",
			output:   filepath.Join(dir, "tls-legacy.conf"),
			args:     withPassword("secrets:asconfig:password", "--sa-address", tlsAgent, "--sa-cafile", pki.CAFile),
			contains: []string{legacyCADeprecation},
		},
		{
			name:   "TLS agent, config file",
			output: filepath.Join(dir, "tls-config-file.conf"),
			args:   []string{"generate", "--config-file", tlsConf},
		},
		{
			name: "TLS agent, diff server",
			args: []string{
				"diff", "server", "-h", host, "-U", "admin", "-P", "secrets:asconfig:password",
				"--sa-address", tlsAgent, "--sa-ca-file", pki.CAFile, generated,
			},
		},
		{
			name:   "TLS agent, untrusted CA",
			output: filepath.Join(dir, "tls-untrusted.conf"),
			args: withPassword("secrets:asconfig:password",
				"--sa-address", tlsAgent, "--sa-ca-file", untrusted.CAFile, "--sa-timeout", "5000"),
			contains: []string{"--password", "secrets:asconfig:password"},
			fail:     true,
		},
		{
			name:     "TLS agent without a CA",
			output:   filepath.Join(dir, "tls-plaintext.conf"),
			args:     withPassword("secrets:asconfig:password", "--sa-address", tlsAgent, "--sa-timeout", "5000"),
			contains: []string{"--password", "secrets:asconfig:password"},
			fail:     true,
		},
		{
			name:     "bad [secret-agent] section fails without secrets",
			output:   filepath.Join(dir, "bad-section.conf"),
			args:     []string{"generate", "--config-file", badSectionConf},
			contains: []string{`invalid value \"0\" for sa-port in the [secret-agent] config section`},
			fail:     true,
		},
		{
			name:     "TLS agent, --only-config-file with --config-file",
			output:   filepath.Join(dir, "only-config-file.conf"),
			args:     []string{"generate", "--only-config-file", tlsConf, "--config-file", badSectionConf},
			contains: []string{"--config-file and --only-config-file are mutually exclusive"},
			fail:     true,
		},
		{
			name:   "--no-config-file reads no file",
			output: filepath.Join(dir, "no-config-file.conf"),
			home:   string(badSectionText),
			args:   withPassword("secrets:asconfig:password", "--sa-address", agent, "--no-config-file"),
		},
		{
			name:   "--no-config-file with --config-file",
			output: filepath.Join(dir, "no-and-config-file.conf"),
			args: withPassword("secrets:asconfig:password",
				"--sa-address", agent, "--no-config-file", "--config-file", tlsConf),
			contains: []string{"--config-file and --no-config-file are mutually exclusive"},
			fail:     true,
		},
		{
			name:   "--no-config-file with --only-config-file",
			output: filepath.Join(dir, "no-and-only-config-file.conf"),
			args:   []string{"generate", "--no-config-file", "--only-config-file", clusterConf},
			contains: []string{
				"--no-config-file and only-config-file are mutually exclusive option. Please enable only one.",
			},
			fail: true,
		},
	})
}

const tlsServerConf = `service {
    feature-key-file /etc/aerospike/secret/features.conf
}

logging {
    console {
        context any info
    }
}

network {
    tls localhost {
        ca-file /opt/aerospike/work/ca.pem
        cert-file /opt/aerospike/work/server.pem
        key-file /opt/aerospike/work/server.key
    }

    service {
        tls-port 4333
        tls-name localhost
        tls-authenticate-client any
    }

    fabric {
        port 3001
    }

    heartbeat {
        mode mesh
        port 3002
    }
}

security {
}

namespace test {
    memory-size 1G
    replication-factor 1
    storage-engine memory
}
`

// encryptedKey returns keyFile encrypted with password in the legacy PEM
// format the Aerospike tools read.
func encryptedKey(t *testing.T, keyFile, password string) []byte {
	t.Helper()

	data, err := os.ReadFile(keyFile)
	if err != nil {
		t.Fatal(err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		t.Fatalf("no PEM block in %s", keyFile)
	}

	encrypted, err := x509.EncryptPEMBlock(rand.Reader, block.Type, block.Bytes, []byte(password), x509.PEMCipherAES256)
	if err != nil {
		t.Fatal(err)
	}

	return pem.EncodeToMemory(encrypted)
}

// TestSecretAgentTLSServer fetches every TLS option from the Secret Agent and
// logs in to a server that requires a client certificate.
func TestSecretAgentTLSServer(t *testing.T) {
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()

	pki, err := secretagenttest.GeneratePKI(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	read := func(path string) string {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}

		return string(data)
	}

	writeFile := func(path, text string) string {
		if writeErr := os.WriteFile(path, []byte(text), tempFilePermissions); writeErr != nil {
			t.Fatal(writeErr)
		}

		return path
	}

	key := string(encryptedKey(t, pki.ClientKeyFile, keyPassword))
	keyFile := writeFile(filepath.Join(dir, "client.key"), key)
	keyPasswordFile := writeFile(filepath.Join(dir, "key-password.txt"), keyPassword+"\nnot the password\n")

	agent := startSecretAgent(t, dockerClient, nil, map[string]string{
		"ca":                 read(pki.CAFile),
		"cert":               read(pki.ClientCertFile),
		"key":                key,
		"key-password":       keyPassword,
		"wrong-key-password": secretAgentWrongPassword,
	})

	confDir := t.TempDir()
	writeFile(filepath.Join(confDir, "ca.pem"), read(pki.CAFile))
	writeFile(filepath.Join(confDir, "server.pem"), read(pki.ServerCertFile))
	writeFile(filepath.Join(confDir, "server.key"), read(pki.ServerKeyFile))
	confPath := writeFile(filepath.Join(confDir, "aerospike.conf"), tlsServerConf)

	addr := startSecretAgentServer(t, dockerClient, confPath, tlsServerPort)
	_, port, _ := net.SplitHostPort(addr)
	host := "127.0.0.1:localhost:" + port

	waitForServer(t, host, dir, "--tls-enable", "--tls-cafile", pki.CAFile,
		"--tls-certfile", pki.ClientCertFile, "--tls-keyfile", keyFile, "--tls-keyfile-password", keyPassword)

	withTLS := func(keyPasswordValue string, args ...string) []string {
		return append([]string{
			"generate", "-h", host, "-U", "admin", "-P", "secrets:asconfig:password", "--sa-address", agent,
			"--tls-enable", "--tls-cafile", "secrets:asconfig:ca", "--tls-certfile", "secrets:asconfig:cert",
			"--tls-keyfile", "secrets:asconfig:key", "--tls-keyfile-password", keyPasswordValue,
		}, args...)
	}

	runSecretAgentCases(t, []secretAgentCase{
		{
			name:   "CA, certificate, key and key password from the Secret Agent",
			output: filepath.Join(dir, "tls.conf"),
			args:   withTLS("secrets:asconfig:key-password"),
		},
		{
			name:     "a wrong key password from the Secret Agent",
			output:   filepath.Join(dir, "tls-wrong-key-password.conf"),
			args:     withTLS("secrets:asconfig:wrong-key-password"),
			contains: []string{"failed to load client authentication certificate and key"},
			fail:     true,
		},
		{
			name:   "surrounding whitespace is trimmed from the key password",
			output: filepath.Join(dir, "tls-trimmed.conf"),
			args:   withTLS(" \tenv:" + keyPasswordEnv + " \n"),
			env:    []string{keyPasswordEnv + "=" + keyPassword},
		},
		{
			name:   "file: key password reads the first line",
			output: filepath.Join(dir, "tls-key-password-file.conf"),
			args:   withTLS("file:" + keyPasswordFile),
		},
	})
}
