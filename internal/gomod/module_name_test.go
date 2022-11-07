package gomod

import "testing"

func TestResolveModuleName(t *testing.T) {
	testCases := []struct {
		repo     string
		name     string
		expected string
	}{
		{
			repo:     "github.com/sourcegraph/sourcegraph",
			name:     "github.com/sourcegraph/sourcegraph",
			expected: "https://github.com/sourcegraph/sourcegraph",
		},
		{
			repo:     "github.com/sourcegraph/zoekt", // forked repo
			name:     "github.com/google/zoekt",      // declared module
			expected: "https://github.com/sourcegraph/zoekt",
		},

		{
			repo:     "github.com/sourcegraph/zoekt",
			name:     "github.com/google/zoekt/some/sub/path",
			expected: "https://github.com/sourcegraph/zoekt/some/sub/path",
		},

		{
			repo:     "github.com/golang/go",
			name:     "std",
			expected: "https://github.com/golang/go",
		},
	}

	for _, testCase := range testCases {
		if actual, _, err := resolveModuleName(testCase.repo, testCase.name); err != nil {
			t.Fatalf("unexpected error: %s", err)
		} else if actual != testCase.expected {
			t.Errorf("unexpected module name. want=%q have=%q", testCase.expected, actual)
		}
	}
}
