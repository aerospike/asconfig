package main

import (
	"aerospike/asconfig/testutils"
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// global flags
var obfuscate *bool

type versions struct {
	TestedVersion         string
	OriginallyUsedVersion string
}

type obfuscateCallback func(*obfuscateEntry, []byte) ([]byte, error)

type obfuscateEntry struct {
	pattern *regexp.Regexp
	value   string
	cb      obfuscateCallback
}

func (o *obfuscateEntry) obfuscateLine(line []byte) ([]byte, error) {
	match := o.pattern.FindSubmatchIndex(line)
	if match == nil {
		return line, nil
	}

	res := o.pattern.ReplaceAll(line, []byte("${1}"+o.value))

	if o.cb != nil {
		return o.cb(o, res)
	}

	return res, nil
}

func obfuscateNamespaceCallback(o *obfuscateEntry, line []byte) ([]byte, error) {
	chop := len(strconv.Itoa(namespacesSeen))
	namespacesSeen = namespacesSeen + 1
	o.value = o.value[:len(o.value)-chop] + strconv.Itoa(namespacesSeen)

	return line, nil
}

func obfuscateMountCallback(o *obfuscateEntry, line []byte) ([]byte, error) {
	chop := len(strconv.Itoa(mountSeen))
	mountSeen = mountSeen + 1
	o.value = o.value[:len(o.value)-chop] + strconv.Itoa(mountSeen)

	return line, nil
}

func obfuscateSetCallback(o *obfuscateEntry, line []byte) ([]byte, error) {
	chop := len(strconv.Itoa(setSeen))
	setSeen = setSeen + 1
	o.value = o.value[:len(o.value)-chop] + strconv.Itoa(setSeen)

	return line, nil
}

func obfuscateDcCallback(o *obfuscateEntry, line []byte) ([]byte, error) {
	chop := len(strconv.Itoa(dcSeen))
	dcSeen = dcSeen + 1
	o.value = o.value[:len(o.value)-chop] + strconv.Itoa(dcSeen)

	return line, nil
}

func obfuscateDeviceCallback(o *obfuscateEntry, line []byte) ([]byte, error) {
	chop := len(strconv.Itoa(deviceSeen))
	deviceSeen = deviceSeen + 1
	o.value = o.value[:len(o.value)-chop] + strconv.Itoa(deviceSeen)

	return line, nil
}

func obfuscateFilePathCallback(o *obfuscateEntry, line []byte) ([]byte, error) {
	chop := len(strconv.Itoa(fileSeen))
	fileSeen = fileSeen + 1
	o.value = o.value[:len(o.value)-chop] + strconv.Itoa(fileSeen)

	return line, nil
}

func obfuscateTLSClusterName(o *obfuscateEntry, line []byte) ([]byte, error) {
	chop := len(strconv.Itoa(tlsSeen))
	tlsSeen = tlsSeen + 1
	o.value = o.value[:len(o.value)-chop] + strconv.Itoa(tlsSeen)

	return line, nil
}

var obfuscateThese = []*obfuscateEntry{
	{
		pattern: regexp.MustCompile(`(namespace\s+)(\S+)`),
		value:   "ns1",
		cb:      obfuscateNamespaceCallback,
	},
	{
		pattern: regexp.MustCompile(`(dc\s+)(\S+)`),
		value:   "dc1",
		cb:      obfuscateDcCallback,
	},
	{
		pattern: regexp.MustCompile(`(xdr-remote-datacenter\s+)(\S+)`),
		value:   "dc1",
		cb:      obfuscateDcCallback,
	},
	{
		pattern: regexp.MustCompile(`((?:^|^\s*)datacenter\s+)(\S+)`),
		value:   "dc1",
		cb:      obfuscateDcCallback,
	},
	{
		pattern: regexp.MustCompile(`(set\s+)(\S+)`),
		value:   "set1",
		cb:      obfuscateSetCallback,
	},
	{
		pattern: regexp.MustCompile(`(cluster-name\s+)(\S+)`),
		value:   "the_cluster_name",
		cb:      nil,
	},
	{
		pattern: regexp.MustCompile(`(address.*\s+)(.{6,})`),
		value:   "127.0.0.1",
		cb:      nil,
	},
	{
		pattern: regexp.MustCompile(`(mount\s+)(\S+)`),
		value:   "/dummy/mount/point1",
		cb:      obfuscateMountCallback,
	},
	{
		pattern: regexp.MustCompile(`(device\s+)([^{].+)`),
		value:   "/dummy/device1",
		cb:      obfuscateDeviceCallback,
	},
	{
		pattern: regexp.MustCompile(`(file\s+)([^{][\S]+)`),
		value:   "/dummy/file/path1",
		cb:      obfuscateFilePathCallback,
	},
	// // set all file logging to /var/log/aerospike/aerospike.log so that it works
	// // in the test containers with default log paths
	// {
	// 	pattern: regexp.MustCompile(`(file\s+)(\S+\s+{)`),
	// 	value:   "/var/log/aerospike/aerospike.log {",
	// 	cb:      nil,
	// },
	{
		pattern: regexp.MustCompile(`((?:^|^\s*)tls\s+)(\S+)`),
		value:   "tls_cluster_name1",
		cb:      obfuscateTLSClusterName,
	},
	{
		pattern: regexp.MustCompile(`(user\s+)(\S+)`),
		value:   "root",
		cb:      nil,
	},
	{
		pattern: regexp.MustCompile(`(^\s*group\s+)(\S+)`),
		value:   "root",
		cb:      nil,
	},
	{
		pattern: regexp.MustCompile(`(multicast-group\s+)(\S+)`),
		value:   "127.0.0.1",
		cb:      nil,
	},
	{
		pattern: regexp.MustCompile(`(tls-name\s+)(\S+)`),
		value:   "tls1",
		cb:      nil,
	},
	{
		pattern: regexp.MustCompile(`(query-base-dn\s+)(\S+)`),
		value:   "dc=dc1,dc=dc2,dc=dc3",
		cb:      nil,
	},
	{
		pattern: regexp.MustCompile(`(server\s+)(\S+)`),
		value:   "ldaps://test.test_server",
		cb:      nil,
	},
	{
		pattern: regexp.MustCompile(`(query-user-dn\s+)(\S+)`),
		value:   "CN=ldapcn,OU=service,DC=dc1,DC=dc2",
		cb:      nil,
	},
	{
		pattern: regexp.MustCompile(`(query-user-password-file\s+)(.*)`),
		value:   "/dummy/pw/file",
		cb:      nil,
	},
	{
		pattern: regexp.MustCompile(`(role-query-pattern\s+)(\S+)`),
		value:   "(&(objectClass=group)(member=${dn}))",
		cb:      nil,
	},
	{
		pattern: regexp.MustCompile(`(user-dn-pattern\s+)(\S+)`),
		value:   "uid=test,ou=Test,dc=datacenter,dc=datacenter2",
		cb:      nil,
	},
	{
		pattern: regexp.MustCompile(`(cert-file\s+)(\S+)`),
		value:   "/x/aerospike/x509_certificates/dummy_cert",
		cb:      nil,
	},
	{
		pattern: regexp.MustCompile(`(key-file\s+)(\S+)`),
		value:   "/x/aerospike/x509_certificates/dummy_key",
		cb:      nil,
	},
	{
		pattern: regexp.MustCompile(`(ca-file\s+)(\S+)`),
		value:   "/x/aerospike/x509_certificates/dummy_ca",
		cb:      nil,
	},
	{
		pattern: regexp.MustCompile(`(tls-node\s+)(\S+\s+\S+\s+\S+)`),
		value:   "127.0.0.1 tls-name 4000",
		cb:      nil,
	},
	{
		pattern: regexp.MustCompile(`(address-port\s+)(\S+(?:\s+\S+)?)`),
		value:   "test_dns_name 4000",
		cb:      nil,
	},
	{
		pattern: regexp.MustCompile(`(feature-key-file\s+)(\S+)`),
		value:   "/etc/aerospike/features.conf",
		cb:      obfuscateFilePathCallback,
	},
	{
		pattern: regexp.MustCompile(`(ca-path\s+)(\S+)`),
		value:   "/path/to/ca",
		cb:      nil,
	},
	{
		pattern: regexp.MustCompile(`(key-file-password\s+)(\S+)`),
		value:   "file:/security/aerospike/keypwd.txt",
		cb:      nil,
	},
	{
		pattern: regexp.MustCompile(`(vault-ca\s+)(\S+)`),
		value:   "/path/to/vault-ca",
		cb:      nil,
	},
	{
		pattern: regexp.MustCompile(`(vault-url\s+)(\S+)`),
		value:   "https://vaulttools",
		cb:      nil,
	},
	{
		pattern: regexp.MustCompile(`(vault-path\s+)(\S+)`),
		value:   "/path/to/vault",
		cb:      nil,
	},
	{
		pattern: regexp.MustCompile(`(http-url\s+)(\S+)`),
		value:   "http://test-dc-url",
		cb:      nil,
	},
}

var namespacesSeen int = 1
var mountSeen int = 1
var setSeen int = 1
var dcSeen int = 1
var deviceSeen int = 1
var fileSeen int = 1
var tlsSeen int = 1

func processFileData(in io.Reader) (io.Reader, error) {
	processedData := bytes.Buffer{}
	scanner := bufio.NewScanner(in)
	for scanner.Scan() {
		tmp := scanner.Bytes()
		// inefficient but this is just a test gen tool
		line := make([]byte, len(tmp))
		copy(line, tmp)

		if i := bytes.IndexRune(line, '#'); i >= 0 {
			line = line[:i]
			// ignore comment only lines
			if len(line) == 0 {
				continue
			}
		}

		if *obfuscate {
			for _, obfs := range obfuscateThese {
				var err error
				line, err = obfs.obfuscateLine(line)
				if err != nil {
					return nil, err
				}
			}
		}

		line = append(line, '\n')
		if _, err := processedData.Write(line); err != nil {
			return nil, err
		}
	}

	return &processedData, nil
}

func main() {
	output := flag.String("output", "./testdata/cases", "path to output directory")
	overwrite := flag.Bool("overwrite", false, "if a testcase directory already exists for this input, overwrite it")
	obfuscate = flag.Bool("obfuscate", false, "obfuscate sensitive fields in the copied source config file")
	aerospikeVersion := flag.String("aerospike-version", "6.2.0.2", "the aerospike version to pass to asconfig e.g: 6.2.0.2")
	originalVersion := flag.String("original-version", "6.2.0.2", "the aerospike version that was originally used with this config e.g: 6.2.0.2")

	flag.Parse()

	if len(flag.Args()) < 1 {
		log.Fatal("no arguments found, must specify path to source conf or yaml file")
	}

	inputPath := flag.Args()[0]

	inputName := filepath.Base(strings.TrimSuffix(inputPath, filepath.Ext(inputPath)))
	testCasePath := filepath.Join(*output, inputName)

	if _, err := os.Stat(testCasePath); !errors.Is(err, os.ErrNotExist) {
		if !*overwrite {
			log.Fatalf("test case for %s already exists", testCasePath)
		}

		err := os.RemoveAll(testCasePath)
		if err != nil {
			log.Fatalf("failed to remove directory %s %v", testCasePath, err)
		}
	}

	err := os.Mkdir(testCasePath, 0755)
	if err != nil {
		log.Fatalf("failed to create directory %s", testCasePath)
	}

	// move source file into testcase dir
	r, err := os.Open(inputPath)
	if err != nil {
		log.Fatalf("failed to open %s", inputPath)
	}
	defer r.Close()

	processedFile, err := processFileData(r)
	if err != nil {
		log.Fatalf("failed to write to processedData %v", err)
	}

	copiedSrcPath := filepath.Join(testCasePath, filepath.Base(inputPath))
	w, err := os.Create(copiedSrcPath)
	if err != nil {
		log.Fatalf("failed to create %s", copiedSrcPath)
	}
	defer w.Close()

	_, err = w.ReadFrom(processedFile)
	if err != nil {
		log.Fatalf("failed to copy %s to %s", inputPath, copiedSrcPath)
	}

	// convert the input file to yaml or asconf
	ext := filepath.Ext(inputPath)
	args := []string{
		"convert",
		copiedSrcPath,
		"-a",
		*aerospikeVersion,
		"--output",
		testCasePath,
	}

	switch ext {
	case ".yaml":
		args = append(args, "--format", "yaml")
	case ".conf":
		args = append(args, "--format", "asconfig")
	default:
		log.Fatalf("Invalid source type: %s, extension must be .yaml, or .conf", ext)
	}

	cmd := exec.Command("asconfig", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("command failed to run %v, %+v, out: %s", cmd, err, string(out))
	}

	// generate basic test cases

	yamlPath := filepath.Join(testCasePath, inputName+".yaml")
	confPath := filepath.Join(testCasePath, inputName+".conf")
	outYamlPath := filepath.Join(testCasePath, inputName+"-res-.conf")
	outConfPath := filepath.Join(testCasePath, inputName+"-res-.yaml")

	// yaml to conf test
	args = []string{
		"convert",
		"--aerospike-version",
		*aerospikeVersion,
		"--format",
		"yaml",
		"--output",
		outYamlPath,
	}

	td := []testutils.TestData{
		{
			Source:      yamlPath,
			Expected:    confPath,
			Destination: outYamlPath,
			Arguments:   args,
		},
	}

	data, err := json.Marshal(td)
	if err != nil {
		log.Fatalf("failed to marshal %v to json, %v", td, err)
	}

	yamlTestPath := filepath.Join(testCasePath, "yaml-tests.json")
	err = os.WriteFile(yamlTestPath, data, 0655)
	if err != nil {
		log.Fatalf("failed to write to %s", yamlTestPath)
	}

	// conf to yaml test
	args = []string{
		"convert",
		"--aerospike-version",
		*aerospikeVersion,
		"--format",
		"asconfig",
		"--output",
		outConfPath,
	}

	td = []testutils.TestData{
		{
			Source:      confPath,
			Expected:    yamlPath,
			Destination: outConfPath,
			Arguments:   args,
		},
	}

	data, err = json.Marshal(td)
	if err != nil {
		log.Fatalf("failed to marshal %v to json, %v", td, err)
	}

	confTestPath := filepath.Join(testCasePath, "conf-tests.json")
	err = os.WriteFile(confTestPath, data, 0655)
	if err != nil {
		log.Fatalf("failed to write to %s", confTestPath)
	}

	// write versions file
	versionsPath := filepath.Join(testCasePath, "versions.json")
	vs := versions{
		TestedVersion:         *aerospikeVersion,
		OriginallyUsedVersion: *originalVersion,
	}

	data, err = json.Marshal(vs)

	err = os.WriteFile(versionsPath, data, 0655)
	if err != nil {
		log.Fatalf("failed to write to %s", versionsPath)
	}

	fmt.Println("Done")
}
