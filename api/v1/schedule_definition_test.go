package v1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScheduleDefinition_String(t *testing.T) {
	assert.Equal(t, "*/5 * * * *", ScheduleDefinition("*/5 * * * *").String())
	assert.Equal(t, "", ScheduleDefinition("").String())
}

func TestScheduleDefinition_IsNonStandard(t *testing.T) {
	tests := map[string]struct {
		input    ScheduleDefinition
		expected bool
	}{
		"standard cron":  {input: "*/5 * * * *", expected: false},
		"@daily":         {input: "@daily", expected: true},
		"@weekly-random": {input: "@weekly-random", expected: true},
		"@hourly-random": {input: "@hourly-random", expected: true},
		"empty":          {input: "", expected: false},
		"just @":         {input: "@", expected: true},
		"no @ prefix":    {input: "daily", expected: false},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.input.IsNonStandard())
		})
	}
}

func TestScheduleDefinition_IsRandom(t *testing.T) {
	tests := map[string]struct {
		input    ScheduleDefinition
		expected bool
	}{
		"@daily-random":  {input: "@daily-random", expected: true},
		"@weekly-random": {input: "@weekly-random", expected: true},
		"@daily":         {input: "@daily", expected: false},
		"standard cron":  {input: "*/5 * * * *", expected: false},
		"empty":          {input: "", expected: false},
		"random no @":    {input: "daily-random", expected: false},
		"just -random":   {input: "-random", expected: false},
		"@-random":       {input: "@-random", expected: true},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.input.IsRandom())
		})
	}
}
