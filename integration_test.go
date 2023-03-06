package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type testFile struct {
	source      string
	destination string
	expected    string
	arguments   []string
}

const (
	binaryPath      = "testdata/asconfig"
	sourcePath      = "testdata/sources"
	expectedPath    = "testdata/expected"
	destinationPath = "testdata/destinations"
)

func TestMain(m *testing.M) {
	if _, err := os.Stat(destinationPath); errors.Is(err, os.ErrNotExist) {
		err := os.Mkdir(destinationPath, 0777)
		if err != nil {
			panic(err)
		}
	}

	code := m.Run()

	err := os.RemoveAll(destinationPath)
	if err != nil {
		panic(err)
	}

	os.Exit(code)
}

var testFiles = []testFile{
	{
		source:      filepath.Join(sourcePath, "config.yaml"),
		destination: filepath.Join(destinationPath, "config.conf"),
		expected:    filepath.Join(expectedPath, "config.conf"),
		arguments:   []string{binaryPath, "-a", "6.2.0.2"},
	},
}

func TestRootCommand(t *testing.T) {
	for _, tf := range testFiles {
		var err error

		arguments := append(tf.arguments, tf.source, tf.destination)
		oldArgs := os.Args[:]
		os.Args = arguments
		main()
		os.Args = oldArgs

		expected, err := os.ReadFile(tf.expected)
		if err != nil {
			t.Error(err)
		}

		actual, err := os.ReadFile(tf.destination)
		if err != nil {
			t.Error(err)
		}

		if string(actual) != string(expected) {
			t.Errorf("\nACTUAL:\n%s\nEXPECTED:\n%s", actual, expected)
		}
	}
}
