//go:build integration
// +build integration

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
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
	sourcePath       = "testdata/sources"
	expectedPath     = "testdata/expected"
	destinationPath  = "testdata/destinations"
	coveragePath     = "testdata/coverage/integration"
	binPath          = "testdata/bin"
	extraTestPath    = "testdata/cases"
	tmpServerLogPath = "testdata/tmp_server.log"
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

	tmpf, err := os.Create(tmpServerLogPath)
	if err != nil {
		panic(err)
	}
	tmpf.Close()

	compileArgs := []string{"build", "-cover", "-coverpkg", "./...", "-o", binPath + "/asconfig.test"}
	compile := exec.Command("go", compileArgs...)
	_, err = compile.CombinedOutput()
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

	if featKeyPath == "" {
		panic("FEATKEY environement variable must be full path to a valid aerospike feature key file")
	}

	code := m.Run()

	err = os.RemoveAll(destinationPath)
	if err != nil {
		panic(err)
	}

	err = os.Remove(tmpServerLogPath)
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
		Arguments:   []string{"convert", "-a", "5.3.0.16", "-o", destinationPath},
	},
	{
		Source:      filepath.Join(sourcePath, "xdr_src_cluster_cr.yaml"),
		Destination: filepath.Join(destinationPath, "xdr_src_cluster_cr.conf"),
		Expected:    filepath.Join(expectedPath, "xdr_src_cluster_cr.conf"),
		Arguments:   []string{"convert", "-a", "6.3.0.6", "-o", filepath.Join(destinationPath, "xdr_src_cluster_cr.conf")},
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

func getVersion(l []string) (v string) {
	i := testutils.IndexOf(l, "-a")
	if i >= 0 {
		v = l[i+1]
	}

	i = testutils.IndexOf(l, "--aerospike-version")
	if i >= 0 {
		v = l[i+1]
	}

	numbs := strings.Split(v, ".")
	major, err := strconv.Atoi(numbs[0])

	if err != nil {
		return
	}

	minor, err := strconv.Atoi(numbs[1])

	if err != nil {
		return
	}

	if major == 5 && minor < 3 || major < 5 {
		return
	}

	return "ee-" + v
}

func TestYamlToConf(t *testing.T) {
	dockerClient, _ := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())

	extraTests, err := getExtraTests(extraTestPath, "yaml")
	if err != nil {
		t.Logf("Skipping getExtraTests: %v", err)
	}

	testFiles = append(testFiles, extraTests...)

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

		confPath := tf.Destination

		// if asconfig wrote to stdout, write a temp file for the server to use
		if confPath == os.Stdout.Name() {
			confPath = filepath.Join(destinationPath, fmt.Sprintf("tmp_stdout_%d.conf", i))
			os.WriteFile(confPath, actual, 0644)
		}

		if _, err := diff(confPath, tf.Expected); err != nil {
			t.Errorf("\nTESTCASE: %+v\nERR: %+v\n", tf, err)
		}

		// test that the converted config works with an Aerospike server
		if !tf.SkipServerTest {
			version := getVersion(tf.Arguments)
			runServer(version, confPath, dockerClient, t, tf)
		}

		// cleanup the destination file
		if err := os.Remove(confPath); err != nil {
			t.Error(err)
		}
	}
}

func getExtraTests(path string, testType string) (tf []testutils.TestData, err error) {
	files, err := os.ReadDir(path)
	if err != nil {
		return
	}

	for _, f := range files {
		tfName := filepath.Join(path, f.Name(), fmt.Sprintf("%s-tests.json", testType))
		fdata, err := os.ReadFile(tfName)
		if err != nil {
			return nil, err
		}

		var testCases []testutils.TestData
		err = json.Unmarshal(fdata, &testCases)
		if err != nil {
			return nil, err
		}

		tf = append(tf, testCases...)
	}

	return
}

func runServer(version string, confPath string, dockerClient *client.Client, t *testing.T, td testutils.TestData) {
	var err error
	containerName := "aerospike:" + version
	serverConfPath := "/opt/aerospike/work/" + filepath.Base(confPath)
	cmd := fmt.Sprintf("/usr/bin/asd --foreground --config-file %s", serverConfPath)
	// cmd = fmt.Sprintf("/bin/bash")

	containerConf := &container.Config{
		Image: containerName,
		Tty:   true,
		Cmd:   strings.Split(cmd, " "),
	}

	destDir := filepath.Dir(confPath)
	absDestPath, err := filepath.Abs(destDir)
	if err != nil {
		t.Error(err)
	}

	absTmpLog, err := filepath.Abs(tmpServerLogPath)
	if err != nil {
		t.Error(err)
	}

	// wipe the tmp server log file inbetween runs
	tmpf, err := os.Create(absTmpLog)
	if err != nil {
		t.Error(err)
	}
	tmpf.Close()

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
			{
				Type:   mount.TypeBind,
				Source: featKeyPath,
				Target: "/etc/aerospike/features.conf",
			},
			{
				Type:   mount.TypeBind,
				Source: absTmpLog,
				Target: "/var/log/aerospike/aerospike.log",
			},
			{
				Type:   mount.TypeBind,
				Source: absTmpLog,
				Target: "/run/log/aerospike.log",
			},
		},
	}

	platform := &v1.Platform{
		Architecture: "amd64",
	}

	// TODO if possible, don't create new containers each time
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

	defer logReader.Close()

	// need this to allow logs to accumulate
	time.Sleep(time.Second * 3)

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

	data, err := io.ReadAll(logReader)
	if err != nil {
		t.Error(err)
	}

	var containerOut string
	containerOut = string(data)

	// containerOut := string(logs)
	// if the server container logs are empty
	// the server may have been configured to log to
	// /var/log/aerospike/aerospike.log which is mapped
	// to absTmpLog
	if len(containerOut) == 0 {
		data, err := os.ReadFile(absTmpLog)
		if err != nil {
			t.Error(err)
		}
		containerOut = string(data)
	}

	// if the logs are still empty, the server logged somewhere else
	// or there is a problem, fail in this case
	if len(containerOut) == 0 {
		t.Errorf("suite: %+v\nAerospike container logs are empty", td)
	}

	// some tests use aerospike versions from when no enterprise container was published
	// these will fail with "'x feature' is enterprise-only"
	// always ignore this failure
	td.ServerErrorAllowList = append(td.ServerErrorAllowList, "' is enterprise-only")

	// TODO support both feature key versions for testing
	// servers older than 5.4 won't accept version 2 feature key files. Suppress this for now
	td.ServerErrorAllowList = append(td.ServerErrorAllowList, " invalid value 2 for feature feature-key-version")

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
			t.Errorf("suite: %+v\nAerospike encountered a configuration error...\n%s", td, containerOut)
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

	extraTests, err := getExtraTests(extraTestPath, "conf")
	if err != nil {
		t.Logf("Skipping getExtraTests: %v", err)
	}

	confToYamlTests = append(confToYamlTests, extraTests...)

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

		// convert the produced yaml file back to conf
		// check that it matches the source and that it
		// works with the server

		confPath := tf.Destination

		// if asconfig wrote to stdout, write a temp file for the server to use
		if confPath == os.Stdout.Name() {
			confPath = filepath.Join(destinationPath, "tmp.yaml")
			os.WriteFile(confPath, actual, 0644)
		}

		// verify that the generated yaml matches the expected yaml
		if _, err := diff(confPath, tf.Expected); err != nil {
			t.Errorf("\nTESTCASE: %+v\nERR: %+v\n", tf, err)
		}

		finalConfPath := filepath.Join(destinationPath, "tmp.conf")
		test = exec.Command(binPath+"/asconfig.test", "convert", "-f", "--format", "yaml", "--output", finalConfPath, confPath)
		test.Env = []string{"GOCOVERDIR=" + coveragePath}
		_, err = test.Output()
		if err != nil {
			t.Error(string(err.(*exec.ExitError).Stderr))
		}

		// verify that the generated conf matches the source conf
		if _, err := diff(tf.Source, finalConfPath); err != nil {
			t.Errorf("\nTESTCASE: %+v\nERR: %+v\n", tf, err)
		}

		// test that the converted config works with an Aerospike server
		if !tf.SkipServerTest {
			version := getVersion(tf.Arguments)
			runServer(version, finalConfPath, dockerClient, t, tf)
		}

		// cleanup the destination files
		if err := os.Remove(finalConfPath); err != nil {
			t.Error(err)
		}

		if err := os.Remove(confPath); err != nil {
			t.Error(err)
		}
	}
}

func diff(args ...string) ([]byte, error) {
	args = append([]string{"diff"}, args...)
	com := exec.Command(binPath+"/asconfig.test", args...)
	com.Env = []string{"GOCOVERDIR=" + coveragePath}
	diff, err := com.Output()
	if err != nil {
		err = fmt.Errorf("diff failed err: %s, out: %s", err, string(diff))
	}
	return diff, err
}

type diffTest struct {
	path1          string
	path2          string
	expectError    bool
	expectedResult string
}

var diffTests = []diffTest{
	{
		path1:          filepath.Join(sourcePath, "pmem_cluster_cr.yaml"),
		path2:          filepath.Join(sourcePath, "pmem_cluster_cr.yaml"),
		expectError:    false,
		expectedResult: "",
	},
	{
		path1:       filepath.Join(sourcePath, "pmem_cluster_cr.yaml"),
		path2:       filepath.Join(sourcePath, "ldap_cluster_cr.yaml"),
		expectError: true,
		expectedResult: `Differences shown from testdata/sources/pmem_cluster_cr.yaml to testdata/sources/ldap_cluster_cr.yaml, '<' are from file1, '>' are from file2.
<: namespaces.{test}.index-type.mounts
<: namespaces.{test}.index-type.mounts-size-limit
<: namespaces.{test}.index-type.type
<: namespaces.{test}.storage-engine.files
<: namespaces.{test}.storage-engine.filesize
namespaces.{test}.storage-engine.type:
	<: pmem
	>: memory
<: network.fabric.port
>: network.fabric.tls-name
>: network.fabric.tls-port
<: network.heartbeat.port
>: network.heartbeat.tls-name
>: network.heartbeat.tls-port
<: network.service.port
>: network.service.tls-authenticate-client
>: network.service.tls-name
>: network.service.tls-port
>: network.tls.{aerospike-a-0.test-runner}.ca-file
>: network.tls.{aerospike-a-0.test-runner}.cert-file
>: network.tls.{aerospike-a-0.test-runner}.key-file
>: network.tls.{aerospike-a-0.test-runner}.name
<: security
>: security.ldap.disable-tls
>: security.ldap.polling-period
>: security.ldap.query-base-dn
>: security.ldap.query-user-dn
>: security.ldap.query-user-password-file
>: security.ldap.role-query-patterns
>: security.ldap.role-query-search-ou
>: security.ldap.server
>: security.ldap.user-dn-pattern

`,
	},
}

func TestDiff(t *testing.T) {
	for _, tf := range diffTests {
		output, err := diff(tf.path1, tf.path2)

		if tf.expectError == (err == nil) {
			t.Errorf("\nTESTCASE: %+v\nERR: %+v\n", tf.path1, err)
		}

		if string(output) != tf.expectedResult {
			t.Errorf("\nTESTCASE: %+v\nACTUAL: %s\nEXPECTED: %s", tf.path1, output, tf.expectedResult)
		}

	}
}

var testStdinConvert = []testutils.TestData{
	{
		Source:    filepath.Join(sourcePath, "all_flash_cluster_cr.yaml"),
		Expected:  filepath.Join(expectedPath, "all_flash_cluster_cr.conf"),
		Arguments: []string{"convert", "-a", "6.2.0.2"},
	},
	{
		Source:    filepath.Join(expectedPath, "all_flash_cluster_cr.conf"),
		Expected:  filepath.Join(sourcePath, "all_flash_cluster_cr.yaml"),
		Arguments: []string{"convert", "-a", "6.2.0.2", "--format", "conf"},
	},
}

func TestConvertStdin(t *testing.T) {
	for _, tf := range testStdinConvert {
		in, err := os.Open(tf.Source)
		if err != nil {
			t.Error(err)
		}
		defer in.Close()

		tmpOutFileName := filepath.Join(destinationPath, "stdinConvertTmp")

		tf.Arguments = append(tf.Arguments, tf.Source, "-o", tmpOutFileName)
		com := exec.Command(binPath+"/asconfig.test", tf.Arguments...)
		com.Env = []string{"GOCOVERDIR=" + coveragePath}
		com.Stdin = in
		output, err := com.Output()
		if err != nil {
			t.Errorf("convert failed err: %s, out: %s", err, string(output))
		}

		diffFormat := filepath.Ext(tf.Expected)
		diffFormat = strings.TrimPrefix(diffFormat, ".")

		if _, err := diff(tmpOutFileName, tf.Expected, "--format", diffFormat); err != nil {
			t.Errorf("\nTESTCASE: %+v\nERR: %+v\n", tf.Source, err)
		}

	}
}
