package main

import (
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/mitchellh/cli"
)

func TestMain_cliArgsFromEnv(t *testing.T) {
	// Setup the state. This test really messes with the environment and
	// global state so we set things up to be restored.

	// Restore original CLI args
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	// Setup test command and restore that
	testCommandName := "unit-test-cli-args"
	testCommand := &testCommandCLI{}
	defer func() { delete(Commands, testCommandName) }()
	Commands[testCommandName] = func() (cli.Command, error) {
		return testCommand, nil
	}

	cases := []struct {
		Name     string
		Args     []string
		Value    string
		Expected []string
		Err      bool
	}{
		{
			"no env",
			[]string{testCommandName, "foo", "bar"},
			"",
			[]string{"foo", "bar"},
			false,
		},

		{
			"both env var and CLI",
			[]string{testCommandName, "foo", "bar"},
			"-foo bar",
			[]string{"-foo", "bar", "foo", "bar"},
			false,
		},

		{
			"only env var",
			[]string{testCommandName},
			"-foo bar",
			[]string{"-foo", "bar"},
			false,
		},

		{
			// this should fail gracefully, this is just testing
			// that we don't panic with our slice arithmetic
			"no command",
			[]string{},
			"-foo bar",
			nil,
			true,
		},

		{
			"single quoted strings",
			[]string{testCommandName, "foo"},
			"-foo 'bar baz'",
			[]string{"-foo", "bar baz", "foo"},
			false,
		},

		{
			"double quoted strings",
			[]string{testCommandName, "foo"},
			`-foo "bar baz"`,
			[]string{"-foo", "bar baz", "foo"},
			false,
		},

		{
			"double quoted single quoted strings",
			[]string{testCommandName, "foo"},
			`-foo "'bar baz'"`,
			[]string{"-foo", "'bar baz'", "foo"},
			false,
		},
	}

	for i, tc := range cases {
		t.Run(fmt.Sprintf("%d-%s", i, tc.Name), func(t *testing.T) {
			os.Unsetenv(EnvCLI)

			// Set the env var value
			if tc.Value != "" {
				if err := os.Setenv(EnvCLI, tc.Value); err != nil {
					t.Fatalf("err: %s", err)
				}
			}

			// Setup the args
			args := make([]string, len(tc.Args)+1)
			args[0] = oldArgs[0] // process name
			copy(args[1:], tc.Args)

			// Run it!
			os.Args = args
			testCommand.Args = nil
			exit := wrappedMain()
			if (exit != 0) != tc.Err {
				t.Fatalf("bad: %d", exit)
			}
			if tc.Err {
				return
			}

			// Verify
			if !reflect.DeepEqual(testCommand.Args, tc.Expected) {
				t.Fatalf("bad: %#v", testCommand.Args)
			}
		})
	}
}

type testCommandCLI struct {
	Args []string
}

func (c *testCommandCLI) Run(args []string) int {
	c.Args = args
	return 0
}

func (c *testCommandCLI) Synopsis() string { return "" }
func (c *testCommandCLI) Help() string     { return "" }
