//go:build integration

package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	as "github.com/aerospike/aerospike-client-go/v7"
	mgmtLib "github.com/aerospike/aerospike-management-lib"
	mgmtLibInfo "github.com/aerospike/aerospike-management-lib/info"
	"github.com/aerospike/asconfig/cmd"
	"github.com/aerospike/asconfig/conf/metadata"
	"github.com/aerospike/asconfig/testutils"
	"github.com/go-logr/logr"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
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

var featKeyDir string

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

	featKeyDir = os.Getenv("FEATKEY_DIR")
	fmt.Println(featKeyDir)
	if featKeyDir == "" {
		panic("FEATKEY_DIR environement variable must an absolute path to a directory containing valid aerospike feature key files featuresv1.conf and featuresv2.conf of feature key format 1 and 2 respectively.")
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
		Arguments:   []string{"convert", "-a", "5.6.0.13", "-o", destinationPath},
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

	testFiles = append(extraTests, testFiles...)

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
			id, _ := runServer(version, tf.ServerImage, confPath, tf.DockerAuth, dockerClient, t)

			time.Sleep(time.Second * 3) // need this to allow logs to accumulate
			checkContainerLogs(id, t, tf, tmpServerLogPath)

			stopServer(id, dockerClient)
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

func getDockerAuthFromEnv(auth testutils.DockerAuth) (string, error) {

	if auth.Password == "" || auth.Username == "" {
		return "", nil
	}

	username := os.Getenv(auth.Username)
	if username == "" {
		return "", fmt.Errorf("docker username environment variable: %s is not set", auth.Username)
	}

	password := os.Getenv(auth.Password)
	if password == "" {
		return "", fmt.Errorf("docker password environment variable: %s is not set", auth.Password)
	}

	parsedAuth := testutils.DockerAuth{
		Username: username,
		Password: password,
	}

	authConfigJSON, err := json.Marshal(parsedAuth)
	if err != nil {
		return "", err
	}

	authStr := base64.URLEncoding.EncodeToString(authConfigJSON)

	return authStr, nil
}

func runServer(version string, serverVersion string, confPath string, auth testutils.DockerAuth, dockerClient *client.Client, t *testing.T) (string, string) {
	containerName := "aerospike:" + version
	if serverVersion != "" {
		containerName = serverVersion
	}

	var err error
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

	featureKeyPath := filepath.Join(featKeyDir, "featuresv2.conf")
	lastServerWithFeatureKeyVersion1 := "5.3.0"

	if val, err := mgmtLib.CompareVersionsIgnoreRevision(strings.TrimPrefix(version, "ee-"), lastServerWithFeatureKeyVersion1); err != nil {
		t.Error(err)
	} else if val <= 0 {
		featureKeyPath = filepath.Join(featKeyDir, "featuresv1.conf")
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
				Source: featureKeyPath,
				Target: "/etc/aerospike/secret/features.conf",
			},
			{
				Type:   mount.TypeBind,
				Source: featureKeyPath,
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

	dockerAuth, err := getDockerAuthFromEnv(auth)
	if err != nil {
		t.Error(err)
	}

	imagePullOptions := types.ImagePullOptions{
		Platform:     "amd64",
		RegistryAuth: dockerAuth,
	}

	// TODO if possible, don't create new containers each time
	id, err := testutils.CreateAerospikeContainer(containerName, containerConf, containerHostConf, imagePullOptions, dockerClient)
	if err != nil {
		t.Error(err)
		return "", ""
	}

	ctx := context.Background()

	err = testutils.StartAerospikeContainer(id, dockerClient)

	if err != nil {
		t.Error(err)
		return "", ""
	}

	resp, err := dockerClient.ContainerInspect(ctx, id)
	if err != nil {
		t.Error(err)
		return "", ""
	}

	return id, resp.NetworkSettings.IPAddress
}

func stopServer(id string, dockerClient *client.Client) error {
	err := testutils.StopAerospikeContainer(id, dockerClient)
	if err != nil {
		return err
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
			return err // TODO: Check if I need to do something to shutdown the channels.
		}
	case <-statusCh:
	}

	err = testutils.RemoveAerospikeContainer(id, dockerClient)
	if err != nil {
		return err
	}

	return nil
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
		Arguments:   []string{"convert", "-a", "6.3.0.6", "--format", "asconfig", "-o", filepath.Join(destinationPath, "xdr_src_cluster_cr.yaml")},
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
			t.Errorf("\nTESTCASE: %+v\nERR: %+v\n", tf, string(err.(*exec.ExitError).Stderr))
		}

		var actual []byte
		if tf.Destination == os.Stdout.Name() {
			actual = out
		} else {
			actual, err = os.ReadFile(tf.Destination)
			if err != nil {
				t.Errorf("\nTESTCASE: %+v\nERR: %+v\n", tf, err)
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
			t.Errorf("\nTESTCASE: %+v\nERR: %+v\n", tf, string(err.(*exec.ExitError).Stderr))
		}

		// verify that the generated conf matches the source conf
		if _, err := diff(tf.Source, finalConfPath); err != nil {
			t.Errorf("\nTESTCASE: %+v\nERR: %+v\n", tf, err)
		}

		// test that the converted config works with an Aerospike server
		if !tf.SkipServerTest {
			version := getVersion(tf.Arguments)
			id, _ := runServer(version, tf.ServerImage, finalConfPath, tf.DockerAuth, dockerClient, t)

			time.Sleep(time.Second * 3) // need this to allow logs to accumulate
			checkContainerLogs(id, t, tf, tmpServerLogPath)

			stopServer(id, dockerClient)
		}

		// cleanup the destination files
		if err := os.Remove(finalConfPath); err != nil {
			t.Errorf("\nTESTCASE: %+v\nERR: %+v\n", tf, err)
		}

		if err := os.Remove(confPath); err != nil {
			t.Errorf("\nTESTCASE: %+v\nERR: %+v\n", tf, err)
		}
	}
}

func docker(args ...string) ([]byte, error) {
	com := exec.Command("docker", args...)
	out, err := com.Output()
	if err != nil {
		err = fmt.Errorf("docker failed err: %s, out: %s", err, string(out))
	}
	return out, err
}

func getContainerLogs(id string) ([]byte, error) {
	return docker("logs", id)
}

func checkContainerLogs(id string, t *testing.T, td testutils.TestData, absTmpLog string) {
	data, err := getContainerLogs(id)
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

func diff(args ...string) ([]byte, error) {
	args = append([]string{"diff"}, args...)
	com := exec.Command(binPath+"/asconfig.test", args...)
	com.Env = []string{"GOCOVERDIR=" + coveragePath}
	diff, err := com.CombinedOutput()
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

type convertMetaDataTest struct {
	expectedMetaData map[string]string
	expectedFile     string
	arguments        []string
}

var convertMetaDataTests = []convertMetaDataTest{
	{
		expectedMetaData: map[string]string{
			"aerospike-server-version": "6.2.0.2",
			"asconfig-version":         cmd.VERSION,
		},
		expectedFile: filepath.Join(expectedPath, "all_flash_cluster_cr.conf"),
		arguments:    []string{"convert", "-a", "6.2.0.2", filepath.Join(sourcePath, "all_flash_cluster_cr.yaml")},
	},
	{
		expectedMetaData: map[string]string{
			"aerospike-server-version": "6.4.0.1",
			"asconfig-version":         cmd.VERSION,
			"asadm-version":            "2.12.0",
		},
		expectedFile: filepath.Join(extraTestPath, "metadata", "metadata.yaml"),
		arguments:    []string{"convert", filepath.Join(extraTestPath, "metadata", "metadata.conf")},
	},
}

func TestConvertMetaData(t *testing.T) {
	for _, tf := range convertMetaDataTests {
		tmpOutFileName := filepath.Join(destinationPath, "stdinConvertTmp")

		tf.arguments = append(tf.arguments, "-o", tmpOutFileName)
		com := exec.Command(binPath+"/asconfig.test", tf.arguments...)
		com.Env = []string{"GOCOVERDIR=" + coveragePath}
		output, err := com.CombinedOutput()
		if err != nil {
			t.Errorf("convert failed err: %s, out: %s", err, string(output))
		}

		fileOut, err := os.ReadFile(tmpOutFileName)
		if err != nil {
			t.Error(err)
		}

		metaData := map[string]string{}
		err = metadata.Unmarshal(fileOut, metaData)
		if err != nil {
			t.Errorf("metadata unmarshal err: %s, out: %s", err, string(fileOut))
		}

		if !reflect.DeepEqual(metaData, tf.expectedMetaData) {
			t.Errorf("METADATA NOT EQUAL\nCASE: %+v\nACTUAL: %+v\nEXPECTED: %+v\n", tf, metaData, tf.expectedMetaData)
		}

		diffFormat := filepath.Ext(tf.expectedFile)
		diffFormat = strings.TrimPrefix(diffFormat, ".")

		if _, err := diff(tmpOutFileName, tf.expectedFile, "--format", diffFormat); err != nil {
			t.Errorf("\nTESTCASE: %+v\nERR: %+v\n", tf, err)
		}
	}
}

type validateTest struct {
	arguments      []string
	source         string
	expectError    bool
	expectedResult string
}

var validateTests = []validateTest{
	{
		arguments:      []string{"validate", "-a", "6.2.0", "-l", "panic", filepath.Join(sourcePath, "pmem_cluster_cr.yaml")},
		expectError:    false,
		expectedResult: "",
		source:         filepath.Join(sourcePath, "pmem_cluster_cr.yaml"),
	},
	{
		arguments:      []string{"validate", "-l", "panic", filepath.Join(extraTestPath, "metadata", "metadata.conf")},
		expectError:    false,
		expectedResult: "",
		source:         filepath.Join(extraTestPath, "metadata", "metadata.conf"),
	},
	{
		arguments:   []string{"validate", "-a", "7.0.0", "-l", "panic", filepath.Join(extraTestPath, "server64", "server64.yaml")},
		expectError: true,
		source:      filepath.Join(extraTestPath, "server64", "server64.yaml"),
		expectedResult: `context: namespaces.ns1
	- description: Additional property memory-size is not allowed, error-type: additional_property_not_allowed
context: namespaces.ns1.index-type
	- description: Additional property mounts-high-water-pct is not allowed, error-type: additional_property_not_allowed
	- description: Additional property mounts-size-limit is not allowed, error-type: additional_property_not_allowed
	- description: mounts-budget is required, error-type: required
context: namespaces.ns1.sindex-type
	- description: Additional property mounts-high-water-pct is not allowed, error-type: additional_property_not_allowed
	- description: Additional property mounts-size-limit is not allowed, error-type: additional_property_not_allowed
	- description: mounts-budget is required, error-type: required
context: namespaces.ns1.storage-engine
	- description: devices is required, error-type: required
context: namespaces.ns2
	- description: Additional property memory-size is not allowed, error-type: additional_property_not_allowed
context: namespaces.ns2.storage-engine
	- description: devices is required, error-type: required
context: service
	- description: cluster-name is required, error-type: required
`,
	},
}

func TestValidate(t *testing.T) {
	for _, tf := range validateTests {
		com := exec.Command(binPath+"/asconfig.test", tf.arguments...)
		com.Env = []string{"GOCOVERDIR=" + coveragePath}
		out, err := com.CombinedOutput()
		if tf.expectError == (err == nil) {
			t.Errorf("\nTESTCASE: %+v\nERR: %+v\n", tf.arguments, err)
		}

		if string(out) != tf.expectedResult {
			t.Errorf("\nTESTCASE: %+v\nACTUAL: %s\nEXPECTED: %s", tf.arguments, string(out), tf.expectedResult)
		}
	}
}

func TestStdinValidate(t *testing.T) {
	for _, tf := range validateTests {
		in, err := os.Open(tf.source)
		if err != nil {
			t.Error(err)
		}
		defer in.Close()

		com := exec.Command(binPath+"/asconfig.test", tf.arguments...)
		com.Env = []string{"GOCOVERDIR=" + coveragePath}
		com.Stdin = in
		out, err := com.CombinedOutput()
		if tf.expectError == (err == nil) {
			t.Errorf("\nTESTCASE: %+v\nERR: %+v\n", tf.arguments, err)
		}

		if string(out) != tf.expectedResult {
			t.Errorf("\nTESTCASE: %+v\nACTUAL: %s\nEXPECTED: %s", tf.arguments, string(out), tf.expectedResult)
		}
	}
}

type generateTC struct {
	source      string
	destination string
	arguments   []string
	version     string
}

var generateTests = []generateTC{
	{
		source:      filepath.Join(expectedPath, "dim_nostorage_cluster_cr.conf"),
		destination: filepath.Join(destinationPath, "dim_nostorage_cluster_cr.conf"),
		arguments:   []string{"generate", "-h", "<ip>", "-Uadmin", "-Padmin", "--format", "asconfig", "-o", filepath.Join(destinationPath, "dim_nostorage_cluster_cr.conf")},
		version:     "ee-6.4.0.2",
	},
	// Uncomment after https://github.com/aerospike/schemas/pull/6 is merged and
	// the schemas submodule is updated
	// {
	// 	source:      filepath.Join(expectedPath, "hdd_dii_storage_cluster_cr.conf"),
	// 	destination: filepath.Join(destinationPath, "hdd_dii_storage_cluster_cr.conf"),
	// 	arguments:   []string{"generate", "-h", "<ip>", "-Uadmin", "-Padmin", "--format", "asconfig", "-o", filepath.Join(destinationPath, "hdd_dii_storage_cluster_cr.conf")},
	// 	version:     "ee-6.2.0.2",
	// },
	// {
	// 	source:      filepath.Join(expectedPath, "hdd_dim_storage_cluster_cr.conf"),
	// 	destination: filepath.Join(destinationPath, "hdd_dim_storage_cluster_cr.conf"),
	// 	arguments:   []string{"generate", "-h", "<ip>", "-Uadmin", "-Padmin", "--format", "asconfig", "--output", filepath.Join(destinationPath, "hdd_dim_storage_cluster_cr.conf")},
	// 	version:     "ee-6.4.0.2",
	// },
	{
		source:      filepath.Join(expectedPath, "podspec_cr.conf"),
		destination: filepath.Join(destinationPath, "podspec_cr.conf"),
		arguments:   []string{"generate", "-h", "<ip>", "-Uadmin", "-Padmin", "--format", "asconfig", "-o", filepath.Join(destinationPath, "podspec_cr.conf")},
		version:     "ee-6.4.0.2",
	},
	{
		source:      filepath.Join(expectedPath, "shadow_file_cluster_cr.conf"),
		destination: filepath.Join(destinationPath, "shadow_file_cluster_cr.conf"),
		arguments:   []string{"generate", "-h", "<ip>", "-Uadmin", "-Padmin", "--format", "asconfig", "-o", filepath.Join(destinationPath, "shadow_file_cluster_cr.conf")},
		version:     "ee-6.4.0.2",
	},
}

func TestGenerate(t *testing.T) {
	dockerClient, _ := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())

	for _, tf := range generateTests {
		var err error

		id, ip := runServer(tf.version, "", tf.source, testutils.DockerAuth{}, dockerClient, t)

		// Make a copy of tf.firstArgs
		firstArgs := make([]string, len(tf.arguments))
		copy(firstArgs, tf.arguments)

		for idx, arg := range firstArgs {
			if arg == "<ip>" {
				firstArgs[idx] = ip
			}
		}

		time.Sleep(time.Second * 3) // need this to allow aerospike to startup

		test := exec.Command(binPath+"/asconfig.test", firstArgs...)
		test.Env = []string{"GOCOVERDIR=" + coveragePath}
		_, err = test.Output()
		if err != nil {
			t.Errorf("\nTESTCASE: %+v\nERR: %+v\n", firstArgs, string(err.(*exec.ExitError).Stderr))
			return
		}

		asPolicy := as.NewClientPolicy()
		asHost := as.NewHost(ip, 3000)

		asPolicy.User = "admin"
		asPolicy.Password = "admin"

		firstConf, err := mgmtLibInfo.NewAsInfo(logr.Logger{}, asHost, asPolicy).AllConfigs()

		if err != nil {
			t.Errorf("\nTESTCASE: %+v\nERR: %+v\n", tf, err)
		}

		err = stopServer(id, dockerClient)
		if err != nil {
			t.Errorf("\nTESTCASE: %+v\nERR: %+v\n", tf, string(err.(*exec.ExitError).Stderr))
		}

		id, ip = runServer(tf.version, "", tf.destination, testutils.DockerAuth{}, dockerClient, t)

		time.Sleep(time.Second * 3) // need this to allow aerospike to startup

		asHost = as.NewHost(ip, 3000)
		secondConf, err := mgmtLibInfo.NewAsInfo(logr.Logger{}, asHost, asPolicy).AllConfigs()

		if err != nil {
			t.Errorf("\nTESTCASE: %+v\nERR: %+v\n", tf, err)
		}

		err = stopServer(id, dockerClient)
		if err != nil {
			t.Errorf("\nTESTCASE: %+v\nERR: %+v\n", tf, string(err.(*exec.ExitError).Stderr))
		}

		// Compare the config of the two servers
		if !reflect.DeepEqual(firstConf, secondConf) {
			t.Errorf("\nTESTCASE: %+v\ndiff: %+v, %+v\n", tf, firstConf, secondConf)
		}
	}
}
