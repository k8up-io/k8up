package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFlags_Combine(t *testing.T) {
	tests := map[string]struct {
		givenFirstFlags  Flags
		givenSecondFlags Flags
		expectedFlags    Flags
	}{
		"GivenEmptySecond": {
			givenFirstFlags:  map[string][]string{"keyInFirst": {"valueInFirst"}},
			givenSecondFlags: map[string][]string{},
			expectedFlags:    map[string][]string{"keyInFirst": {"valueInFirst"}},
		},
		"GivenEmptyFirst": {
			givenFirstFlags:  map[string][]string{},
			givenSecondFlags: map[string][]string{"keyInSecond": {"valueInSecond"}},
			expectedFlags:    map[string][]string{"keyInSecond": {"valueInSecond"}},
		},
		"GivenNonOverlappingKeys": {
			givenFirstFlags:  map[string][]string{"keyInFirst": {"valueInFirst"}},
			givenSecondFlags: map[string][]string{"keyInSecond": {"valueInSecond"}},
			expectedFlags:    map[string][]string{"keyInFirst": {"valueInFirst"}, "keyInSecond": {"valueInSecond"}},
		},
		"GivenOverlappingKeys": {
			givenFirstFlags:  map[string][]string{"keyInBoth": {"valueInFirst"}},
			givenSecondFlags: map[string][]string{"keyInBoth": {"valueInSecond"}},
			expectedFlags:    map[string][]string{"keyInBoth": {"valueInFirst", "valueInSecond"}},
		},
		"GivenOverlappingKeysAndValues": {
			givenFirstFlags:  map[string][]string{"keyInBoth": {"valueInBoth"}},
			givenSecondFlags: map[string][]string{"keyInBoth": {"valueInBoth"}},
			expectedFlags:    map[string][]string{"keyInBoth": {"valueInBoth", "valueInBoth"}},
		},
		"GivenOverlappingKeysAndEmptyValues": {
			givenFirstFlags:  map[string][]string{"keyInBoth": {}},
			givenSecondFlags: map[string][]string{"keyInBoth": {}},
			expectedFlags:    map[string][]string{"keyInBoth": {}},
		},
		"GivenOverlappingKeysAndEmptyStringValues": {
			givenFirstFlags:  map[string][]string{"keyInBoth": {""}},
			givenSecondFlags: map[string][]string{"keyInBoth": {""}},
			expectedFlags:    map[string][]string{"keyInBoth": {"", ""}},
		},
	}

	for name, params := range tests {
		t.Run(name, func(t *testing.T) {
			actualFlags := Combine(params.givenFirstFlags, params.givenSecondFlags)

			for actualKey, actualValue := range actualFlags {
				assert.ElementsMatchf(t, params.expectedFlags[actualKey], actualValue, "Values of key '%s' are not as expected", actualKey)
			}
		})
	}
}

func TestFlags_ApplyTo(t *testing.T) {
	tests := map[string]struct {
		givenCommand     string
		givenCommandArgs []string
		givenFlags       Flags
		expectedArgs     []string
	}{
		"GivenNothing_ExpectEmptyArray": {
			expectedArgs: []string{},
		},
		"GivenCommand_ExpectJustCommand": {
			givenCommand: "command",
			expectedArgs: []string{"command"},
		},
		"GivenCommandWithCommandArgs_ExpectJustCommandWithCommandArgs": {
			givenCommand:     "command",
			givenCommandArgs: []string{"--command-argument", "parameter"},
			expectedArgs:     []string{"command", "--command-argument", "parameter"},
		},
		"GivenCommandWithFlag_ExpectJustCommandAndFlag": {
			givenCommand: "command",
			givenFlags:   map[string][]string{"--flag": {}},
			expectedArgs: []string{"command", "--flag"},
		},
		"GivenCommandWithCommandArgsAndFlag_ExpectCommandAndCommandArgsAndFlag": {
			givenCommand:     "command",
			givenCommandArgs: []string{"--command-argument", "parameter"},
			givenFlags:       map[string][]string{"--flag": {}},
			expectedArgs:     []string{"command", "--flag", "--command-argument", "parameter"},
		},
		"GivenCommandWithCommandArgsAndFlagWithFlagArgs_ExpectCommandAndCommandArgsAndFlagAndFlagArgs": {
			givenCommand:     "command",
			givenCommandArgs: []string{"--command-argument", "parameter"},
			givenFlags:       map[string][]string{"--flag": {"flag-argument"}},
			expectedArgs:     []string{"command", "--flag", "flag-argument", "--command-argument", "parameter"},
		},
		"GivenCommandWithFlagAndFlagArg_ExpectCommandWithFlagAndFlagArg": {
			givenCommand: "command",
			givenFlags:   map[string][]string{"--flag": {"flag-argument"}},
			expectedArgs: []string{"command", "--flag", "flag-argument"},
		},
		"GivenCommandWithFlagAndEmptyFlagArg_ExpectCommandWithFlagAndEmptyFlagArg": {
			givenCommand: "command",
			givenFlags:   map[string][]string{"--flag": {""}},
			expectedArgs: []string{"command", "--flag", ""},
		},
		"GivenCommandWithFlagAndMultipleFlagArgs_ExpectCommandWithFlagAndArgCombination": {
			givenCommand: "command",
			givenFlags:   map[string][]string{"--flag": {"flag-argument1", "flag-argument2"}},
			expectedArgs: []string{"command", "--flag", "flag-argument1", "--flag", "flag-argument2"},
		},
		"GivenCommandWithTwoFlags_ExpectCommandWithTwoFlagsAndTheirArguments": {
			givenCommand: "command",
			givenFlags:   map[string][]string{"--flag1": {"flag-argument"}, "--flag2": {}},
			expectedArgs: []string{"command", "--flag1", "flag-argument", "--flag2"},
		},
		"GivenCommandWithCommandArgsAndMultipleFlagsWithMultipleFlagArgs_ExpectCorrectCommand": {
			givenCommand:     "command",
			givenCommandArgs: []string{"--command-argument", "parameter"},
			givenFlags:       map[string][]string{"--flag1": {"flag-argument1", "flag-argument2"}, "--flag2": {""}, "--flag3": {}},
			expectedArgs:     []string{"command", "--flag1", "flag-argument1", "--flag1", "flag-argument2", "--flag2", "", "--flag3", "--command-argument", "parameter"},
		},
	}

	for name, params := range tests {
		t.Run(name, func(t *testing.T) {
			actualArgs := params.givenFlags.ApplyToCommand(params.givenCommand, params.givenCommandArgs...)

			assert.ElementsMatch(t, params.expectedArgs, actualArgs)
		})
	}
}
