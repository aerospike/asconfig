package cmd

import (
	"errors"
	"os"
	"testing"

	"github.com/aerospike/tools-common-go/config"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/suite"
)

func TestPersistentPreRunRootFlags(t *testing.T) {
	var testCases = []struct {
		flags          []string
		arguments      []string
		expectedErrors []error
	}{
		{
			flags:          []string{"-l", "info"},
			arguments:      []string{},
			expectedErrors: []error{nil},
		},
		{
			flags:          []string{"-l", "panic"},
			arguments:      []string{},
			expectedErrors: []error{nil},
		},
		{
			flags:     []string{"--log-level", "bad_level"},
			arguments: []string{},
			expectedErrors: []error{
				errInvalidLogLevel,
			},
		},
	}
	cmd := newRootCmd()

	for _, tc := range testCases {
		t.Run(tc.flags[0], func(t *testing.T) {
			cmd.ParseFlags(tc.flags)
			err := cmd.PersistentPreRunE(cmd, tc.arguments)
			for _, expectedErr := range tc.expectedErrors {
				if !errors.Is(err, expectedErr) {
					t.Errorf("%v\n actual err: %v\n is not expected err: %v", tc.flags, err, expectedErr)
				}
			}
		})
	}
}

const tomlConfigTxt = `
[group1]
str1 = "localhost:3000"
int1 = 3000
bool1 = true

[group2]
str2 = "localhost:4000"
int2 = 4000
bool2 = false
`

const yamlConfigTxt = `
group1:
  str1: "localhost:3000"
  int1:  3000
  bool1:  true

group2:
  str2: "localhost:4000"
  int2: 4000
  bool2: false
`

type RootTest struct {
	suite.Suite
}

func (suite *RootTest) TestPersistentPreRunRootInitConfig() {
	testCases := []struct {
		configFile    string
		configFileTxt string
	}{
		{
			configFile:    "test.conf",
			configFileTxt: tomlConfigTxt,
		},
		{
			configFile:    "test.yaml",
			configFileTxt: yamlConfigTxt,
		},
	}

	createCmd := func() (*cobra.Command, *pflag.FlagSet, *pflag.FlagSet) {
		rootCmd := newRootCmd()
		subCmd := &cobra.Command{
			Use: "sub",
			Run: func(cmd *cobra.Command, args []string) {},
		}
		flagSet1 := &pflag.FlagSet{}
		flagSet2 := &pflag.FlagSet{}

		rootCmd.AddCommand(subCmd)
		flagSet1.String("str1", "str1", "string flag")
		flagSet1.Int("int1", 0, "int flag")
		flagSet1.Bool("bool1", false, "bool flag")
		flagSet2.String("str2", "str2", "string flag")
		flagSet2.Int("int2", 0, "int flag")
		flagSet2.Bool("bool2", false, "bool flag")
		config.BindPFlags(flagSet1, "group1")
		config.BindPFlags(flagSet2, "group2")
		subCmd.PersistentFlags().AddFlagSet(flagSet1)
		subCmd.PersistentFlags().AddFlagSet(flagSet2)

		return rootCmd, flagSet1, flagSet2
	}

	for _, tc := range testCases {
		suite.T().Run(tc.configFile, func(t *testing.T) {
			config.Reset()

			rootCmd, flagSet1, flagSet2 := createCmd()

			err := os.WriteFile(tc.configFile, []byte(tc.configFileTxt), 0600)
			if err != nil {
				t.Fatalf("unable to write %s: %v", tc.configFile, err)
			}

			defer os.Remove(tc.configFile)

			rootCmd.SetArgs([]string{"sub", "--config-file", tc.configFile})
			err = rootCmd.Execute()

			if err != nil {
				suite.FailNow("unexpected error", err)
			}

			str1, err := flagSet1.GetString("str1")
			suite.NoError(err)
			suite.Equal("localhost:3000", str1)

			int1, err := flagSet1.GetInt("int1")
			suite.NoError(err)
			suite.Equal(3000, int1)

			bool1, err := flagSet1.GetBool("bool1")
			suite.NoError(err)
			suite.Equal(true, bool1)

			str2, err := flagSet2.GetString("str2")
			suite.NoError(err)
			suite.Equal("localhost:4000", str2)

			int2, err := flagSet2.GetInt("int2")
			suite.NoError(err)
			suite.Equal(4000, int2)

			bool2, err := flagSet2.GetBool("bool2")
			suite.NoError(err)
			suite.Equal(false, bool2)

		})
	}
}

func TestRunTestSuite(t *testing.T) {
	suite.Run(t, new(RootTest))
}
