package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"aerospike/asconfig/testutils"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	sourcePath      = "testdata/sources"
	expectedPath    = "testdata/expected"
	destinationPath = "testdata/destinations"
	coveragePath    = "testdata/coverage/integration"
	binPath         = "testdata/bin"
)

var featKeyPath string

func TestMain(m *testing.M) {
	if _, err := os.Stat(destinationPath); errors.Is(err, os.ErrNotExist) {
		err := os.Mkdir(destinationPath, 0777)
		if err != nil {
			panic(err)
		}
	}

	if _, err := os.Stat(coveragePath); errors.Is(err, os.ErrNotExist) {
		err := os.Mkdir(coveragePath, 0777)
		if err != nil {
			panic(err)
		}
	}

	compileArgs := []string{"build", "-cover", "-coverpkg", "./...", "-o", binPath + "/asconfig.test"}
	compile := exec.Command("go", compileArgs...)
	_, err := compile.CombinedOutput()
	if err != nil {
		panic(err)
	}

	envVars := os.Environ()
	for _, v := range envVars {
		pair := strings.Split(v, "=")
		if pair[0] == "FEATKEY" {
			featKeyPath = pair[1]
		}
	}
	//TODO REMOVE THIS
	featKeyPath = "/Users/dwelch/Desktop/everything/docs/features.conf"

	code := m.Run()

	err = os.RemoveAll(destinationPath)
	if err != nil {
		panic(err)
	}

	os.Exit(code)
}

var testFiles = []testutils.TestData{
	{
		Source:      filepath.Join(sourcePath, "all_flash_cluster_cr.yaml"),
		Destination: filepath.Join(destinationPath, "all_flash_cluster_cr.conf"),
		Expected:    filepath.Join(expectedPath, "all_flash_cluster_cr.conf"),
		Arguments:   []string{"convert", "-a", "6.2.0.2", "-o", destinationPath},
	},
	{
		Source:      filepath.Join(sourcePath, "multiple_feature_key_paths.yaml"),
		Destination: filepath.Join(destinationPath, "multiple_feature_key_paths.conf"),
		Expected:    filepath.Join(expectedPath, "multiple_feature_key_paths.conf"),
		Arguments:   []string{"convert", "-a", "6.2.0.2", "-o", destinationPath},
	},
	{
		Source:      filepath.Join(sourcePath, "dim_nostorage_cluster_cr.yaml"),
		Destination: filepath.Join(destinationPath, "dim_nostorage_cluster_cr.conf"),
		Expected:    filepath.Join(expectedPath, "dim_nostorage_cluster_cr.conf"),
		Arguments:   []string{"convert", "-a", "6.2.0.2", "-o", destinationPath},
	},
	{
		Source:      filepath.Join(sourcePath, "dim_nostorage_cluster_cr.yaml"),
		Destination: filepath.Join(destinationPath, "dim_nostorage_cluster_cr.conf"),
		Expected:    filepath.Join(expectedPath, "dim_nostorage_cluster_cr.conf"),
		Arguments:   []string{"convert", "-a", "5.3.0.16", "-o", destinationPath},
	},
	{
		Source:      filepath.Join(sourcePath, "dim_nostorage_cluster_cr.yaml"),
		Destination: filepath.Join(destinationPath, "dim_nostorage_cluster_cr.conf"),
		Expected:    filepath.Join(expectedPath, "dim_nostorage_cluster_cr.conf"),
		Arguments:   []string{"convert", "-a", "6.0.0.5", "-o", destinationPath},
	},
	{
		Source:      filepath.Join(sourcePath, "hdd_dii_storage_cluster_cr.yaml"),
		Destination: filepath.Join(destinationPath, "hdd_dii_storage_cluster_cr.conf"),
		Expected:    filepath.Join(expectedPath, "hdd_dii_storage_cluster_cr.conf"),
		Arguments:   []string{"convert", "-a", "6.2.0.2", "-o", destinationPath},
	},
	{
		Source:      filepath.Join(sourcePath, "hdd_dim_storage_cluster_cr.yaml"),
		Destination: filepath.Join(destinationPath, "hdd_dim_storage_cluster_cr.conf"),
		Expected:    filepath.Join(expectedPath, "hdd_dim_storage_cluster_cr.conf"),
		Arguments:   []string{"convert", "-a", "6.2.0.2", "--output", destinationPath},
	},
	{
		Source:      filepath.Join(sourcePath, "host_network_cluster_cr.yaml"),
		Destination: filepath.Join(destinationPath, "host_network_cluster_cr.conf"),
		Expected:    filepath.Join(expectedPath, "host_network_cluster_cr.conf"),
		Arguments:   []string{"convert", "-a", "6.2.0.2", "-o", destinationPath},
	},
	{
		Source:      filepath.Join(sourcePath, "ldap_cluster_cr.yaml"),
		Destination: filepath.Join(destinationPath, "ldap_cluster_cr.conf"),
		Expected:    filepath.Join(expectedPath, "ldap_cluster_cr.conf"),
		Arguments:   []string{"convert", "-a", "6.2.0.2", "-o", destinationPath},
	},
	{
		Source:               filepath.Join(sourcePath, "pmem_cluster_cr.yaml"),
		Destination:          filepath.Join(destinationPath, "pmem_cluster_cr.conf"),
		Expected:             filepath.Join(expectedPath, "pmem_cluster_cr.conf"),
		Arguments:            []string{"convert", "-a", "6.2.0.2", "-o", destinationPath},
		ServerErrorAllowList: []string{"missing or invalid mount point"},
	},
	{
		Source:      filepath.Join(sourcePath, "podspec_cr.yaml"),
		Destination: filepath.Join(destinationPath, "podspec_cr.conf"),
		Expected:    filepath.Join(expectedPath, "podspec_cr.conf"),
		Arguments:   []string{"convert", "-a", "6.2.0.2", "-o", destinationPath},
	},
	{
		Source:      filepath.Join(sourcePath, "rack_enabled_cluster_cr.yaml"),
		Destination: filepath.Join(destinationPath, "rack_enabled_cluster_cr.conf"),
		Expected:    filepath.Join(expectedPath, "rack_enabled_cluster_cr.conf"),
		Arguments:   []string{"convert", "-a", "6.2.0.2", "-o", destinationPath},
	},
	{
		Source:      filepath.Join(sourcePath, "sc_mode_cluster_cr.yaml"),
		Destination: filepath.Join(destinationPath, "sc_mode_cluster_cr.conf"),
		Expected:    filepath.Join(expectedPath, "sc_mode_cluster_cr.conf"),
		Arguments:   []string{"convert", "-a", "6.2.0.2", "-o", destinationPath},
	},
	{
		Source:      filepath.Join(sourcePath, "shadow_device_cluster_cr.yaml"),
		Destination: filepath.Join(destinationPath, "shadow_device_cluster_cr.conf"),
		Expected:    filepath.Join(expectedPath, "shadow_device_cluster_cr.conf"),
		Arguments:   []string{"convert", "-a", "6.2.0.2", "-o", destinationPath},
	},
	{
		Source:      filepath.Join(sourcePath, "shadow_file_cluster_cr.yaml"),
		Destination: filepath.Join(destinationPath, "shadow_file_cluster_cr.conf"),
		Expected:    filepath.Join(expectedPath, "shadow_file_cluster_cr.conf"),
		Arguments:   []string{"convert", "-a", "6.2.0.2", "-o", destinationPath},
	},
	{
		Source:      filepath.Join(sourcePath, "ssd_storage_cluster_cr.yaml"),
		Destination: filepath.Join(destinationPath, "ssd_storage_cluster_cr.conf"),
		Expected:    filepath.Join(expectedPath, "ssd_storage_cluster_cr.conf"),
		Arguments:   []string{"convert", "-a", "6.2.0.2", "-o", destinationPath},
	},
	{
		Source:               filepath.Join(sourcePath, "tls_cluster_cr.yaml"),
		Destination:          filepath.Join(destinationPath, "tls_cluster_cr.conf"),
		Expected:             filepath.Join(expectedPath, "tls_cluster_cr.conf"),
		Arguments:            []string{"convert", "-a", "6.2.0.2", "-o", filepath.Join(destinationPath, "tls_cluster_cr.conf")},
		ServerErrorAllowList: []string{"failed to set up service tls"},
	},
	{
		Source:      filepath.Join(sourcePath, "xdr_dst_cluster_cr.yaml"),
		Destination: filepath.Join(destinationPath, "xdr_dst_cluster_cr.conf"),
		Expected:    filepath.Join(expectedPath, "xdr_dst_cluster_cr.conf"),
		Arguments:   []string{"convert", "-a", "6.2.0.2", "-o", destinationPath},
	},
	{
		Source:      filepath.Join(sourcePath, "xdr_src_cluster_cr.yaml"),
		Destination: filepath.Join(destinationPath, "xdr_src_cluster_cr.conf"),
		Expected:    filepath.Join(expectedPath, "xdr_src_cluster_cr.conf"),
		Arguments:   []string{"convert", "-a", "5.3.0.16", "-o", filepath.Join(destinationPath, "xdr_src_cluster_cr.conf")},
	},
	{
		Source:      filepath.Join(sourcePath, "missing_heartbeat_mode.yaml"),
		Destination: filepath.Join(destinationPath, "missing_heartbeat_mode.conf"),
		Expected:    filepath.Join(expectedPath, "missing_heartbeat_mode.conf"),
		Arguments:   []string{"convert", "-a", "6.2.0.2", "-f", "--output", filepath.Join(destinationPath, "missing_heartbeat_mode.conf")},
	},
	{
		Source:      filepath.Join(sourcePath, "missing_heartbeat_mode.yaml"),
		Destination: filepath.Join(destinationPath, "missing_heartbeat_mode.conf"),
		Expected:    filepath.Join(expectedPath, "missing_heartbeat_mode.conf"),
		Arguments:   []string{"convert", "-a", "6.2.0.2", "-f", "-o", destinationPath},
	},
	{
		Source:      filepath.Join(sourcePath, "missing_heartbeat_mode.yaml"),
		Destination: os.Stdout.Name(),
		Expected:    filepath.Join(expectedPath, "missing_heartbeat_mode.conf"),
		Arguments:   []string{"convert", "-a", "6.2.0.2", "-f"},
	},
}

func getVersion(l []string) string {
	i := testutils.IndexOf(l, "-a")
	if i >= 0 {
		return l[i+1]
	}

	i = testutils.IndexOf(l, "--aerospike-version")
	if i >= 0 {
		return l[i+1]
	}

	return ""
}

func TestYamlToConf(t *testing.T) {
	dockerClient, _ := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())

	for i, tf := range testFiles {
		var err error

		tf.Arguments = append(tf.Arguments, tf.Source)
		test := exec.Command(binPath+"/asconfig.test", tf.Arguments...)
		test.Env = []string{"GOCOVERDIR=" + coveragePath}

		stdout := &bytes.Buffer{}
		test.Stdout = stdout

		stderr := &bytes.Buffer{}
		test.Stderr = stderr

		test.Run()
		if err != nil || len(stderr.Bytes()) > 0 {
			t.Error(stderr.String())
		}

		var actual []byte
		if tf.Destination == os.Stdout.Name() {
			actual = stdout.Bytes()
		} else {
			actual, err = os.ReadFile(tf.Destination)
			if err != nil {
				t.Error(err)
			}
		}

		expected, err := os.ReadFile(tf.Expected)
		if err != nil {
			t.Error(err)
		}

		sortedActual, err := sortLines(actual)
		if err != nil {
			t.Error(err)
		}

		sortedExpected, err := sortLines(expected)
		if err != nil {
			t.Error(err)
		}

		if string(sortedActual) != string(sortedExpected) {
			diff, _ := diff(tf.Destination, tf.Expected)
			t.Errorf("\nTESTCASE: %+v\nACTUAL:\n%s\nEXPECTED:\n%s\nDIFF:\n%s\nERR: %+v\n", tf, actual, expected, string(diff), err)
		}

		// test that the converted config works with an Aerospike server
		confPath := tf.Destination

		// if asconfig wrote to stdout, write a temp file for the server to use
		if confPath == os.Stdout.Name() {
			confPath = filepath.Join(destinationPath, fmt.Sprintf("tmp_stdout_%d.conf", i))
			os.WriteFile(confPath, actual, 0644)
		}

		if !tf.SkipServerTest {
			version := getVersion(tf.Arguments)
			runServer(version, filepath.Base(confPath), dockerClient, t, tf)
		}
	}
}

func sortLines(data []byte) ([]byte, error) {
	r := bufio.NewReader(bytes.NewReader(bytes.Clone(data)))
	var lines []string

	for {
		const delim = '\n'
		line, err := r.ReadString(delim)
		if err == nil || len(line) > 0 {
			if err != nil {
				line += string(delim)
			}
			lines = append(lines, line)
		}

		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
	}

	sort.Strings(lines)
	sorted := strings.Join(lines, "")

	return []byte(sorted), nil
}

func runServer(version string, confName string, dockerClient *client.Client, t *testing.T, td testutils.TestData) {
	var err error
	containerName := "aerospike:ee-" + version
	confPath := "/opt/aerospike/work/" + confName
	cmd := fmt.Sprintf("/usr/bin/asd --foreground --config-file %s", confPath)

	containerConf := &container.Config{
		Image: containerName,
		Tty:   true,
		Cmd:   strings.Split(cmd, " "),
	}

	absDestPath, err := filepath.Abs(destinationPath)
	if err != nil {
		t.Error(err)
	}

	containerHostConf := &container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: absDestPath,
				Target: "/opt/aerospike/work",
			},
			{
				Type:   mount.TypeBind,
				Source: featKeyPath,
				Target: "/etc/aerospike/secret/features.conf",
			},
		},
	}

	platform := &v1.Platform{
		Architecture: "amd64",
	}

	// maybe don't create new containers each time? is it possible to
	id, err := testutils.CreateAerospikeContainer(containerName, containerConf, containerHostConf, platform, dockerClient)
	if err != nil {
		t.Error(err)
	}

	// cleanup container
	defer func() {
		err = testutils.RemoveAerospikeContainer(id, dockerClient)
		if err != nil {
			t.Error(err)
		}
	}()

	err = testutils.StartAerospikeContainer(id, dockerClient)

	if err != nil {
		t.Error(err)
	}

	logReader, err := dockerClient.ContainerLogs(context.Background(), id, types.ContainerLogsOptions{ShowStdout: true, ShowStderr: true, Follow: true})
	if err != nil {
		t.Error(err)
	}

	time.Sleep(time.Second)

	defer logReader.Close()
	data, err := io.ReadAll(logReader)
	if err != nil {
		t.Error(err)
	}

	containerOut := string(data)

	err = testutils.StopAerospikeContainer(id, dockerClient)
	if err != nil {
		t.Error(err)
	}

	// time for Aerospike to close
	statusCh, errCh := dockerClient.ContainerWait(
		context.Background(),
		id,
		container.WaitConditionNotRunning,
	)
	select {
	case err := <-errCh:
		if err != nil {
			t.Error(err)
		}
	case <-statusCh:
	}

	if len(containerOut) == 0 {
		t.Error("Aerospike container logs are empty")
	}

	reg := regexp.MustCompile(`CRITICAL \(config\):.*`)
	configErrors := reg.FindAllString(containerOut, -1)
	for _, cfgErr := range configErrors {
		exempted := false
		for _, exemption := range td.ServerErrorAllowList {
			if strings.Contains(cfgErr, exemption) {
				exempted = true
			}
		}

		if !exempted {
			t.Errorf("Aerospike encountered a configuration error...\n%s", data)
		}
	}
}

var confToYamlTests = []testutils.TestData{
	{
		Source:      filepath.Join(expectedPath, "all_flash_cluster_cr.conf"),
		Destination: filepath.Join(destinationPath, "all_flash_cluster_cr.yaml"),
		Expected:    filepath.Join(sourcePath, "all_flash_cluster_cr.yaml"),
		Arguments:   []string{"convert", "-a", "6.2.0.2", "--format", "asconfig", "-o", destinationPath},
	},
	{
		Source:               filepath.Join(expectedPath, "multiple_tls_authenticate_client.conf"),
		Destination:          filepath.Join(destinationPath, "multiple_tls_authenticate_client.yaml"),
		Expected:             filepath.Join(sourcePath, "multiple_tls_authenticate_client.yaml"),
		Arguments:            []string{"convert", "-a", "6.2.0.2", "--format", "asconfig", "-o", destinationPath},
		ServerErrorAllowList: []string{"failed to set up service tls"},
	},
	{
		Source:      filepath.Join(expectedPath, "dim_nostorage_cluster_cr.conf"),
		Destination: filepath.Join(destinationPath, "dim_nostorage_cluster_cr.yaml"),
		Expected:    filepath.Join(sourcePath, "dim_nostorage_cluster_cr.yaml"),
		Arguments:   []string{"convert", "-a", "6.2.0.2", "--format", "asconfig", "-o", destinationPath},
	},
	{
		Source:      filepath.Join(expectedPath, "dim_nostorage_cluster_cr.conf"),
		Destination: filepath.Join(destinationPath, "dim_nostorage_cluster_cr.yaml"),
		Expected:    filepath.Join(sourcePath, "dim_nostorage_cluster_cr.yaml"),
		Arguments:   []string{"convert", "-a", "5.3.0.16", "--format", "asconfig", "-o", destinationPath},
	},
	{
		Source:      filepath.Join(expectedPath, "dim_nostorage_cluster_cr.conf"),
		Destination: filepath.Join(destinationPath, "dim_nostorage_cluster_cr.yaml"),
		Expected:    filepath.Join(sourcePath, "dim_nostorage_cluster_cr.yaml"),
		Arguments:   []string{"convert", "-a", "6.0.0.5", "--format", "asconfig", "-o", destinationPath},
	},
	{
		Source:      filepath.Join(expectedPath, "hdd_dii_storage_cluster_cr.conf"),
		Destination: filepath.Join(destinationPath, "hdd_dii_storage_cluster_cr.yaml"),
		Expected:    filepath.Join(sourcePath, "hdd_dii_storage_cluster_cr.yaml"),
		Arguments:   []string{"convert", "-a", "6.2.0.2", "--format", "asconfig", "-o", destinationPath},
	},
	{
		Source:      filepath.Join(expectedPath, "hdd_dim_storage_cluster_cr.conf"),
		Destination: filepath.Join(destinationPath, "hdd_dim_storage_cluster_cr.yaml"),
		Expected:    filepath.Join(sourcePath, "hdd_dim_storage_cluster_cr.yaml"),
		Arguments:   []string{"convert", "-a", "6.2.0.2", "--format", "asconfig", "--output", destinationPath},
	},
	{
		Source:      filepath.Join(expectedPath, "host_network_cluster_cr.conf"),
		Destination: filepath.Join(destinationPath, "host_network_cluster_cr.yaml"),
		Expected:    filepath.Join(sourcePath, "host_network_cluster_cr.yaml"),
		Arguments:   []string{"convert", "-a", "6.2.0.2", "--format", "asconfig", "-o", destinationPath},
	},
	{
		Source:      filepath.Join(expectedPath, "ldap_cluster_cr.conf"),
		Destination: filepath.Join(destinationPath, "ldap_cluster_cr.yaml"),
		Expected:    filepath.Join(sourcePath, "ldap_cluster_cr.yaml"),
		Arguments:   []string{"convert", "-a", "6.2.0.2", "--format", "asconfig", "-o", destinationPath},
	},
	{
		Source:               filepath.Join(expectedPath, "pmem_cluster_cr.conf"),
		Destination:          filepath.Join(destinationPath, "pmem_cluster_cr.yaml"),
		Expected:             filepath.Join(sourcePath, "pmem_cluster_cr.yaml"),
		Arguments:            []string{"convert", "-a", "6.2.0.2", "--format", "asconfig", "-o", destinationPath},
		ServerErrorAllowList: []string{"missing or invalid mount point"},
	},
	{
		Source:      filepath.Join(expectedPath, "podspec_cr.conf"),
		Destination: filepath.Join(destinationPath, "podspec_cr.yaml"),
		Expected:    filepath.Join(sourcePath, "podspec_cr.yaml"),
		Arguments:   []string{"convert", "-a", "6.2.0.2", "--format", "asconfig", "-o", destinationPath},
	},
	{
		Source:      filepath.Join(expectedPath, "rack_enabled_cluster_cr.conf"),
		Destination: filepath.Join(destinationPath, "rack_enabled_cluster_cr.yaml"),
		Expected:    filepath.Join(sourcePath, "rack_enabled_cluster_cr.yaml"),
		Arguments:   []string{"convert", "-a", "6.2.0.2", "--format", "asconfig", "-o", destinationPath},
	},
	{
		Source:      filepath.Join(expectedPath, "sc_mode_cluster_cr.conf"),
		Destination: filepath.Join(destinationPath, "sc_mode_cluster_cr.yaml"),
		Expected:    filepath.Join(sourcePath, "sc_mode_cluster_cr.yaml"),
		Arguments:   []string{"convert", "-a", "6.2.0.2", "--format", "asconfig", "-o", destinationPath},
	},
	{
		Source:      filepath.Join(expectedPath, "shadow_device_cluster_cr.conf"),
		Destination: filepath.Join(destinationPath, "shadow_device_cluster_cr.yaml"),
		Expected:    filepath.Join(sourcePath, "shadow_device_cluster_cr.yaml"),
		Arguments:   []string{"convert", "-a", "6.2.0.2", "--format", "asconfig", "-o", destinationPath},
	},
	{
		Source:      filepath.Join(expectedPath, "shadow_file_cluster_cr.conf"),
		Destination: filepath.Join(destinationPath, "shadow_file_cluster_cr.yaml"),
		Expected:    filepath.Join(sourcePath, "shadow_file_cluster_cr.yaml"),
		Arguments:   []string{"convert", "-a", "6.2.0.2", "--format", "asconfig", "-o", destinationPath},
	},
	{
		Source:      filepath.Join(expectedPath, "ssd_storage_cluster_cr.conf"),
		Destination: filepath.Join(destinationPath, "ssd_storage_cluster_cr.yaml"),
		Expected:    filepath.Join(sourcePath, "ssd_storage_cluster_cr.yaml"),
		Arguments:   []string{"convert", "-a", "6.2.0.2", "--format", "asconfig", "-o", destinationPath},
	},
	{
		Source:               filepath.Join(expectedPath, "tls_cluster_cr.conf"),
		Destination:          filepath.Join(destinationPath, "tls_cluster_cr.yaml"),
		Expected:             filepath.Join(sourcePath, "tls_cluster_cr.yaml"),
		Arguments:            []string{"convert", "-a", "6.2.0.2", "--format", "asconfig", "-o", filepath.Join(destinationPath, "tls_cluster_cr.yaml")},
		ServerErrorAllowList: []string{"failed to set up service tls"},
	},
	{
		Source:      filepath.Join(expectedPath, "xdr_dst_cluster_cr.conf"),
		Destination: filepath.Join(destinationPath, "xdr_dst_cluster_cr.yaml"),
		Expected:    filepath.Join(sourcePath, "xdr_dst_cluster_cr.yaml"),
		Arguments:   []string{"convert", "-a", "6.2.0.2", "--format", "asconfig", "-o", destinationPath},
	},
	{
		Source:      filepath.Join(expectedPath, "xdr_src_cluster_cr.conf"),
		Destination: filepath.Join(destinationPath, "xdr_src_cluster_cr.yaml"),
		Expected:    filepath.Join(sourcePath, "xdr_src_cluster_cr.yaml"),
		Arguments:   []string{"convert", "-a", "5.3.0.16", "--format", "asconfig", "-o", filepath.Join(destinationPath, "xdr_src_cluster_cr.yaml")},
	},
	{
		Source:      filepath.Join(expectedPath, "missing_heartbeat_mode.conf"),
		Destination: filepath.Join(destinationPath, "missing_heartbeat_mode.yaml"),
		Expected:    filepath.Join(sourcePath, "missing_heartbeat_mode.yaml"),
		Arguments:   []string{"convert", "-a", "6.2.0.2", "--format", "asconfig", "-f", "--output", filepath.Join(destinationPath, "missing_heartbeat_mode.yaml")},
	},
	{
		Source:      filepath.Join(expectedPath, "missing_heartbeat_mode.conf"),
		Destination: filepath.Join(destinationPath, "missing_heartbeat_mode.yaml"),
		Expected:    filepath.Join(sourcePath, "missing_heartbeat_mode.yaml"),
		Arguments:   []string{"convert", "-a", "6.2.0.2", "--format", "asconfig", "-f", "-o", destinationPath},
	},
	{
		Source:      filepath.Join(expectedPath, "missing_heartbeat_mode.conf"),
		Destination: os.Stdout.Name(),
		Expected:    filepath.Join(sourcePath, "missing_heartbeat_mode.yaml"),
		Arguments:   []string{"convert", "-a", "6.2.0.2", "--format", "asconfig", "-f"},
	},
}

func TestConfToYaml(t *testing.T) {
	dockerClient, _ := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())

	for _, tf := range confToYamlTests {
		var err error

		tf.Arguments = append(tf.Arguments, tf.Source)
		test := exec.Command(binPath+"/asconfig.test", tf.Arguments...)
		test.Env = []string{"GOCOVERDIR=" + coveragePath}
		out, err := test.Output()
		if err != nil {
			t.Error(string(err.(*exec.ExitError).Stderr))
		}

		var actual []byte
		if tf.Destination == os.Stdout.Name() {
			actual = out
		} else {
			actual, err = os.ReadFile(tf.Destination)
			if err != nil {
				t.Error(err)
			}
		}

		expected, err := os.ReadFile(tf.Expected)
		if err != nil {
			t.Error(err)
		}

		sortedActual, err := sortLines(actual)
		if err != nil {
			t.Error(err)
		}

		sortedExpected, err := sortLines(expected)
		if err != nil {
			t.Error(err)
		}

		// diff, err := diff(tf.destination, tf.expected)
		// if err != nil {
		// 	t.Errorf("\nTESTCASE: %+v\nACTUAL:\n%s\nEXPECTED:\n%s\nDIFF:\n%s\nERR: %+v\n", tf, actual, expected, string(diff), err)
		// }

		if string(sortedActual) != string(sortedExpected) {
			diff, _ := diff(tf.Destination, tf.Expected)
			t.Errorf("\nTESTCASE: %+v\nACTUAL:\n%s\nEXPECTED:\n%s\nDIFF:\n%s\nERR: %+v\n", tf, actual, expected, string(diff), err)
		}

		// convert the produced conf file back to yaml
		// check that it matches the source and that it
		// works with the server

		if tf.Destination == os.Stdout.Name() {
			actual = out
		} else {
			actual, err = os.ReadFile(tf.Destination)
			if err != nil {
				t.Error(err)
			}
		}

		confPath := tf.Destination

		// if asconfig wrote to stdout, write a temp file for the server to use
		if confPath == os.Stdout.Name() {
			confPath = filepath.Join(destinationPath, "tmp.yaml")
			os.WriteFile(confPath, actual, 0644)
		}

		test = exec.Command(binPath+"/asconfig.test", "convert", "-f", "--format", "yaml", "--output", filepath.Join(destinationPath, "tmp.conf"), confPath)
		test.Env = []string{"GOCOVERDIR=" + coveragePath}
		_, err = test.Output()
		if err != nil {
			t.Error(string(err.(*exec.ExitError).Stderr))
		}

		if !tf.SkipServerTest {
			version := getVersion(tf.Arguments)
			runServer(version, "tmp.conf", dockerClient, t, tf)
		}
	}
}

func diff(path1 string, path2 string) ([]byte, error) {
	com := exec.Command("diff", path1, path2)
	diff, err := com.Output()
	if err != nil {
		err = fmt.Errorf("diff failed err: %s, out: %s", err, string(diff))
	}
	return diff, err
}
