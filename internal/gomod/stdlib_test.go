package gomod

import "testing"

func TestStdLib(t *testing.T) {
	expectedStdlib := []string{
		"fmt",
		"database/sql",
		"net/http/httptrace",
	}

	for _, testCase := range expectedStdlib {
		if !IsStandardlibPackge(testCase) {
			t.Errorf(`"%s" should be marked as a standard library package`, testCase)
		}
	}

	expectedUserlib := []string{
		"github.com/sourcegraph/lsif-go/internal/command",
		"github.com/sourcegraph/lsif-go/internal/output",
		"myCustomName/hello",
	}

	for _, testCase := range expectedUserlib {
		if IsStandardlibPackge(testCase) {
			t.Errorf(`"%s" should not be marked as a standard library package`, testCase)
		}
	}
}
