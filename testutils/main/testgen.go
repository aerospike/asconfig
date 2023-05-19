package main

import (
	"aerospike/asconfig/testutils"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

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

	copiedSrcPath := filepath.Join(testCasePath, filepath.Base(inputPath))
	w, err := os.Create(copiedSrcPath)
	if err != nil {
		log.Fatalf("failed to create %s", copiedSrcPath)
	}
	defer w.Close()

	_, err = w.ReadFrom(r)
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

	yamlPath := filepath.Join(testCasePath, inputName, ".yaml")
	confPath := filepath.Join(testCasePath, inputName, ".conf")
	resultsPath := filepath.Join(testCasePath, "tmp")

	// yaml to conf test
	args = []string{
		"convert",
		"--aerospike-version",
		*aerospikeVersion,
		"--format",
		"yaml",
		"--output",
		resultsPath,
	}

	td := testutils.TestData{
		Source:      yamlPath,
		Expected:    confPath,
		Destination: filepath.Join(resultsPath, inputName, ".yaml"),
		Arguments:   args,
	}

	data, err := json.Marshal(td)
	if err != nil {
		log.Fatalf("failed to marshal %v to json, %w", td, err)
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
		resultsPath,
	}

	td = testutils.TestData{
		Source:      yamlPath,
		Expected:    confPath,
		Destination: filepath.Join(resultsPath, inputName, ".conf"),
		Arguments:   args,
	}

	data, err = json.Marshal(td)
	if err != nil {
		log.Fatalf("failed to marshal %v to json, %w", td, err)
	}

	confTestPath := filepath.Join(testCasePath, "conf-tests.json")
	err = os.WriteFile(confTestPath, data, 0655)
	if err != nil {
		log.Fatalf("failed to write to %s", confTestPath)
	}

	fmt.Println("Done")
}
