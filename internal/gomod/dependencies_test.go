package gomod

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseGoListOutput(t *testing.T) {
	output := `
		{
			"Path": "github.com/gavv/httpexpect",
			"Version": "v2.0.0+incompatible",
			"Time": "2019-05-23T21:42:28Z",
			"Indirect": true,
			"GoMod": "/Users/efritz/go/pkg/mod/cache/download/github.com/gavv/httpexpect/@v/v2.0.0+incompatible.mod"
		}
		{
			"Path": "github.com/getsentry/raven-go",
			"Version": "v0.2.0",
			"Time": "2018-11-28T22:11:06Z",
			"Dir": "/Users/efritz/go/pkg/mod/github.com/getsentry/raven-go@v0.2.0",
			"GoMod": "/Users/efritz/go/pkg/mod/cache/download/github.com/getsentry/raven-go/@v/v0.2.0.mod"
		}
		{
			"Path": "github.com/gfleury/go-bitbucket-v1",
			"Version": "v0.0.0-20200312180434-e5170e3280fb",
			"Time": "2020-03-12T18:04:34Z",
			"Indirect": true,
			"Dir": "/Users/efritz/go/pkg/mod/github.com/gfleury/go-bitbucket-v1@v0.0.0-20200312180434-e5170e3280fb",
			"GoMod": "/Users/efritz/go/pkg/mod/cache/download/github.com/gfleury/go-bitbucket-v1/@v/v0.0.0-20200312180434-e5170e3280fb.mod",
			"GoVersion": "1.14"
		}
		{
			"Path": "github.com/ghodss/yaml",
			"Version": "v1.0.0",
			"Replace": {
				"Path": "github.com/sourcegraph/yaml",
				"Version": "v1.0.1-0.20200714132230-56936252f152",
				"Time": "2020-07-14T13:22:30Z",
				"Dir": "/Users/efritz/go/pkg/mod/github.com/sourcegraph/yaml@v1.0.1-0.20200714132230-56936252f152",
				"GoMod": "/Users/efritz/go/pkg/mod/cache/download/github.com/sourcegraph/yaml/@v/v1.0.1-0.20200714132230-56936252f152.mod"
			},
			"Dir": "/Users/efritz/go/pkg/mod/github.com/sourcegraph/yaml@v1.0.1-0.20200714132230-56936252f152",
			"GoMod": "/Users/efritz/go/pkg/mod/cache/download/github.com/sourcegraph/yaml/@v/v1.0.1-0.20200714132230-56936252f152.mod"
		}
		{
			"Path": "github.com/sourcegraph/sourcegraph/enterprise/lib",
			"Version": "v0.0.0-00010101000000-000000000000",
			"Replace": {
				"Path": "./enterprise/lib",
				"Dir": "/Users/efritz/dev/sourcegraph/sourcegraph/enterprise/lib",
				"GoMod": "/Users/efritz/dev/sourcegraph/sourcegraph/lib/go.mod",
				"GoVersion": "1.16"
			},
			"Dir": "/Users/efritz/dev/sourcegraph/sourcegraph/enterprise/lib",
			"GoMod": "/Users/efritz/dev/sourcegraph/sourcegraph/lib/go.mod",
			"GoVersion": "1.16"
		}
	`

	modules, err := parseGoListOutput(output, "v1.2.3")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	expected := map[string]GoModule{
		"github.com/golang/go":                              {Name: "github.com/golang/go", Version: "go1.14"},
		"github.com/gavv/httpexpect":                        {Name: "github.com/gavv/httpexpect", Version: "v2.0.0"},
		"github.com/getsentry/raven-go":                     {Name: "github.com/getsentry/raven-go", Version: "v0.2.0"},
		"github.com/gfleury/go-bitbucket-v1":                {Name: "github.com/gfleury/go-bitbucket-v1", Version: "e5170e3280fb"},
		"github.com/ghodss/yaml":                            {Name: "github.com/sourcegraph/yaml", Version: "56936252f152"},
		"github.com/sourcegraph/sourcegraph/enterprise/lib": {Name: "./enterprise/lib", Version: "v1.2.3"},
	}
	if diff := cmp.Diff(expected, modules); diff != "" {
		t.Errorf("unexpected parsed modules (-want +got): %s", diff)
	}
}

func TestCleanVersion(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{input: "v2.25.0", expected: "v2.25.0"},
		{input: "v2.25.0+incompatible", expected: "v2.25.0"},
		{input: "v0.0.0-20190905194746-02993c407bfb", expected: "02993c407bfb"},
	}

	for _, testCase := range testCases {
		if actual := cleanVersion(testCase.input); actual != testCase.expected {
			t.Errorf("unexpected clean version. want=%q have=%q", testCase.expected, actual)
		}
	}
}

func TestResolveImportPaths(t *testing.T) {
	modules := []string{
		"cloud.google.com/go/pubsub",
		"github.com/etcd-io/bbolt",
		"gitlab.com/nyarla/go-crypt",
		"go.etcd.io/etcd",
		"go.uber.org/zap",
		"golang.org/x/crypto",
		"gopkg.in/inf.v0",
		"k8s.io/klog",
		"rsc.io/binaryregexp",
		"rsc.io/quote/v3",
		"./enterprise/lib",
	}

	expected := map[string]string{
		"cloud.google.com/go/pubsub": "https://github.com/googleapis/google-cloud-go/pubsub",
		"github.com/etcd-io/bbolt":   "https://github.com/etcd-io/bbolt",
		"gitlab.com/nyarla/go-crypt": "https://gitlab.com/nyarla/go-crypt.git",
		"go.etcd.io/etcd":            "https://github.com/etcd-io/etcd",
		"go.uber.org/zap":            "https://github.com/uber-go/zap",
		"golang.org/x/crypto":        "https://go.googlesource.com/crypto",
		"gopkg.in/inf.v0":            "https://gopkg.in/inf.v0",
		"k8s.io/klog":                "https://github.com/kubernetes/klog",
		"rsc.io/binaryregexp":        "https://github.com/rsc/binaryregexp",
		"rsc.io/quote/v3":            "https://github.com/rsc/quote/v3",
		"./enterprise/lib":           "https://github.com/sourcegraph/sourcegraph/enterprise/lib",
	}
	if diff := cmp.Diff(expected, resolveImportPaths("https://github.com/sourcegraph/sourcegraph", modules)); diff != "" {
		t.Errorf("unexpected import paths (-want +got): %s", diff)
	}
}
