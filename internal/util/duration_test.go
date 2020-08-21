package util

import (
	"testing"
	"time"
)

func TestHumanElapsed(t *testing.T) {
	testCases := []struct {
		input    time.Duration
		expected time.Duration
	}{
		{
			input:    time.Nanosecond * 123,
			expected: time.Nanosecond * 123,
		},

		{
			input:    time.Microsecond*5 + time.Nanosecond*123,
			expected: time.Microsecond*5 + time.Nanosecond*120,
		},
		{
			input:    time.Millisecond*5 + time.Microsecond*123 + time.Nanosecond*123,
			expected: time.Millisecond*5 + time.Microsecond*120,
		},
		{
			input:    time.Second*5 + time.Millisecond*123 + time.Microsecond*123,
			expected: time.Second*5 + time.Millisecond*120,
		},
		{
			input:    time.Minute*5 + time.Second*12 + time.Millisecond*123,
			expected: time.Minute*5 + time.Second*12,
		},
		{
			input:    time.Hour*5 + time.Minute*12 + time.Second*12,
			expected: time.Hour*5 + time.Minute*12,
		},
	}

	for _, testCase := range testCases {
		if actual := humanElapsed(testCase.input); actual != testCase.expected {
			t.Errorf("unexpected duration for %s. want=%q have=%q", testCase.input, testCase.expected, actual)
		}
	}
}
