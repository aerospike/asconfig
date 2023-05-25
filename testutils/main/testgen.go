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

type obfuscateCallback func(*obfuscateEntry, []byte) ([]byte, error)

type obfuscateEntry struct {
	pattern *regexp.Regexp
	value   string
	cb      obfuscateCallback
}

func (o *obfuscateEntry) obfuscateLine(line []byte) ([]byte, error) {
	match := o.pattern.Find(line)
	if match == nil {
		return line, nil
	}

	pair := strings.Split(string(match), " ")
	pair[1] = o.value

	matchIndexes := o.pattern.FindIndex(line)
	tmpLine := line[:matchIndexes[0]]
	newVal := strings.Join(pair, " ")
	tmpLine = append(tmpLine, []byte(newVal)...)
	line = append(tmpLine, line[matchIndexes[1]:]...)

	if o.cb != nil {
		return o.cb(o, line)
	}

	return line, nil
}

func obfuscateNamespaceCallback(o *obfuscateEntry, line []byte) ([]byte, error) {
	namespacesSeen = namespacesSeen + 1
	o.value = o.value[:2] + strconv.Itoa(namespacesSeen)

	return line, nil
}

var obfuscateThese = []*obfuscateEntry{
	{
		pattern: regexp.MustCompile(`namespace \S+`),
		value:   "ns1",
		cb:      obfuscateNamespaceCallback,
	},
	{
		pattern: regexp.MustCompile(`address.* \S+`),
		value:   "127.0.0.1",
		cb:      nil,
	},
	{
		pattern: regexp.MustCompile(`mount \S+`),
		value:   "/dummy/mount/point",
		cb:      nil,
	},
	{
		pattern: regexp.MustCompile(`device [^{][\S]+`),
		value:   "/dummy/device",
		cb:      nil,
	},
	{
		pattern: regexp.MustCompile(`file [^{][\S]+`),
		value:   "/dummy/file/path",
		cb:      nil,
	},
	{
		pattern: regexp.MustCompile(`user \S+`),
		value:   "root",
		cb:      nil,
	},
	{
		pattern: regexp.MustCompile(`group \S+`),
		value:   "root",
		cb:      nil,
	},
}

var namespacesSeen int = 1

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

		for _, obfs := range obfuscateThese {
			var err error
			line, err = obfs.obfuscateLine(line)
			if err != nil {
				return nil, err
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
	aerospikeVersion := flag.String("aerospike-version", "6.2.0", "the aerospike version to pass to asconfig e.g: 6.2.0")

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
	outConfPath := filepath.Join(testCasePath, inputName+"-res-.conf")

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
			Source:      yamlPath,
			Expected:    confPath,
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

	fmt.Println("Done")
}
