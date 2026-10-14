//go:build integration

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aerospike/tools-common-go/secretagent"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"

	"github.com/aerospike/asconfig/testutils"
)

const (
	secretAgentImage         = "aerospike/aerospike-secret-agent:1.1.0"
	secretAgentPlatform      = "linux/amd64"
	secretAgentServerVersion = "ee-6.4.0.2"
	secretAgentPassword      = "admin"
	secretAgentWrongPassword = "not-the-admin-password"
	secretAgentStartTimeout  = 2 * time.Minute
)

// startSecretAgent runs a Secret Agent with a file backend holding the server
// password, and returns its address once it serves secrets.
func startSecretAgent(t *testing.T, dockerClient *client.Client) string {
	t.Helper()

	ctx := context.Background()

	if _, err := dockerClient.ImageInspect(ctx, secretAgentImage); err != nil {
		rc, pullErr := dockerClient.ImagePull(ctx, secretAgentImage, image.PullOptions{Platform: secretAgentPlatform})
		if pullErr != nil {
			t.Fatalf("pull %s: %v", secretAgentImage, pullErr)
		}

		_, pullErr = io.Copy(io.Discard, rc)
		rc.Close()

		if pullErr != nil {
			t.Fatalf("pull %s: %v", secretAgentImage, pullErr)
		}
	}

	dir := t.TempDir()

	secrets, err := json.Marshal(map[string]string{
		"password": secretAgentPassword,
		"wrong":    secretAgentWrongPassword,
	})
	if err != nil {
		t.Fatal(err)
	}

	agentConfig := "service:\n  tcp:\n    endpoint: 0.0.0.0:3005\n" +
		"secret-manager:\n  file:\n    convert-to-base64: true\n    resources:\n" +
		"      asconfig: \"/agent/asconfig.json\"\n" +
		"log:\n  level: debug\n"

	// World-readable so the agent can read them whatever uid Docker maps it to.
	for name, data := range map[string][]byte{"asconfig.json": secrets, "agent.yaml": []byte(agentConfig)} {
		if err := os.WriteFile(filepath.Join(dir, name), data, tempFilePermissions); err != nil {
			t.Fatal(err)
		}
	}

	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	created, err := dockerClient.ContainerCreate(ctx,
		&container.Config{Image: secretAgentImage, Cmd: []string{"--config-file", "/agent/agent.yaml"}},
		&container.HostConfig{
			Mounts: []mount.Mount{{Type: mount.TypeBind, Source: dir, Target: "/agent", ReadOnly: true}},
		},
		nil, nil, "")
	if err != nil {
		t.Fatalf("create secret agent container: %v", err)
	}

	t.Cleanup(func() {
		_ = dockerClient.ContainerRemove(context.Background(), created.ID, container.RemoveOptions{Force: true})
	})

	if err = dockerClient.ContainerStart(ctx, created.ID, container.StartOptions{}); err != nil {
		t.Fatalf("start secret agent container: %v", err)
	}

	info, err := dockerClient.ContainerInspect(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}

	addr := net.JoinHostPort(containerIP(info), "3005")

	// The agent logs that it listens before its file backend has loaded, so
	// wait until it actually serves the secret.
	agentClient, err := secretagent.NewClient(secretagent.Config{Address: addr, Base64: true, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(secretAgentStartTimeout)

	for {
		value, resolveErr := agentClient.Resolve(ctx, "secrets:asconfig:password")
		if resolveErr == nil && value == secretAgentPassword {
			return addr
		}

		if time.Now().After(deadline) {
			t.Fatalf("secret agent at %s never served secrets: %v", addr, resolveErr)
		}

		time.Sleep(250 * time.Millisecond)
	}
}

func runAsconfig(args ...string) (string, error) {
	cmd := exec.Command(binPath+"/asconfig.test", args...)
	cmd.Env = []string{"GOCOVERDIR=" + coveragePath}

	var output bytes.Buffer

	cmd.Stdout = &output
	cmd.Stderr = &output

	err := cmd.Run()

	return output.String(), err
}

// waitForServer retries generate with a literal password until the server
// accepts it.
func waitForServer(t *testing.T, ip, dir string) {
	t.Helper()

	out := filepath.Join(dir, "baseline.conf")
	deadline := time.Now().Add(time.Minute)

	for {
		output, err := runAsconfig("generate", "-h", ip, "-U", "admin", "-P", secretAgentPassword, "-o", out)
		if err == nil {
			return
		}

		if time.Now().After(deadline) {
			t.Fatalf("server at %s never became ready: %v\n%s", ip, err, output)
		}

		time.Sleep(time.Second)
	}
}

func TestSecretAgent(t *testing.T) {
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		t.Fatal(err)
	}

	agent := startSecretAgent(t, dockerClient)
	agentHost, agentPort, _ := net.SplitHostPort(agent)

	source := filepath.Join(expectedPath, "dim_nostorage_cluster_cr.conf")

	id, ip := runServer(secretAgentServerVersion, "", source, nil, testutils.DockerAuth{}, dockerClient, t)
	if id == "" {
		t.FailNow()
	}

	t.Cleanup(func() {
		if err := stopServer(id, dockerClient); err != nil {
			t.Errorf("stop server: %v", err)
		}
	})

	dir := t.TempDir()
	waitForServer(t, ip, dir)

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
`, ip, agent))

	instanceConf := writeConf("astools-instance.conf", fmt.Sprintf(`[cluster_prod]
host = "%s"
user = "admin"
password = "secrets:asconfig:password"

[secret-agent]
sa-address = "%s"
`, ip, agent))

	generated := filepath.Join(dir, "generated.conf")

	tests := []struct {
		name     string
		output   string
		args     []string
		contains []string
		fail     bool
	}{
		{
			name:   "generate, password before --sa-address",
			output: generated,
			args:   []string{"generate", "-h", ip, "-U", "admin", "-P", "secrets:asconfig:password", "--sa-address", agent},
		},
		{
			name:   "generate, --sa-address before password",
			output: filepath.Join(dir, "order.conf"),
			args:   []string{"generate", "--sa-address", agent, "-h", ip, "-U", "admin", "-P", "secrets:asconfig:password"},
		},
		{
			name:   "generate, --sa-address and --sa-port",
			output: filepath.Join(dir, "port.conf"),
			args: []string{
				"generate", "-h", ip, "-U", "admin", "-P", "secrets:asconfig:password",
				"--sa-address", agentHost, "--sa-port", agentPort,
			},
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
				"diff", "server", "-h", ip, "-U", "admin", "-P", "secrets:asconfig:password",
				"--sa-address", agent, generated,
			},
		},
		{
			name:     "missing secret",
			args:     []string{"generate", "-h", ip, "-U", "admin", "-P", "secrets:asconfig:missing", "--sa-address", agent},
			contains: []string{"--password", "secrets:asconfig:missing"},
			fail:     true,
		},
		{
			name:     "agent unreachable",
			args:     []string{"generate", "-h", ip, "-U", "admin", "-P", "secrets:asconfig:password", "--sa-address", "127.0.0.1:1"},
			contains: []string{"--password", "secrets:asconfig:password"},
			fail:     true,
		},
		{
			name: "fetched value is used as the password",
			args: []string{"generate", "-h", ip, "-U", "admin", "-P", "secrets:asconfig:wrong", "--sa-address", agent},
			fail: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			args := tc.args
			if tc.output != "" {
				args = append(args, "-o", tc.output)
			}

			output, err := runAsconfig(args...)

			if tc.fail && err == nil {
				t.Fatalf("expected asconfig to fail:\n%s", output)
			}

			if !tc.fail && err != nil {
				t.Fatalf("asconfig failed: %v\n%s", err, output)
			}

			for _, want := range tc.contains {
				if !strings.Contains(output, want) {
					t.Errorf("output does not contain %q:\n%s", want, output)
				}
			}

			if strings.Contains(output, secretAgentWrongPassword) {
				t.Errorf("output contains a secret value:\n%s", output)
			}

			if tc.output != "" {
				data, readErr := os.ReadFile(tc.output)
				if readErr != nil || !strings.Contains(string(data), "namespace test") {
					t.Errorf("unexpected generated config (%v):\n%s", readErr, data)
				}
			}
		})
	}
}
