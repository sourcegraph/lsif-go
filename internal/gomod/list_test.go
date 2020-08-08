package gomod

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

const testGoListOutput = `
github.com/sourcegraph/lsif-go
github.com/alecthomas/kingpin v2.2.6+incompatible
github.com/alecthomas/template v0.0.0-20190718012654-fb15b899a751
github.com/alecthomas/units v0.0.0-20190924025748-f65c72e2690d
github.com/davecgh/go-spew v1.1.1
github.com/efritz/pentimento v0.0.0-20190429011147-ade47d831101
github.com/google/go-cmp v0.5.1
github.com/kr/pretty v0.1.0
github.com/kr/pty v1.1.1
github.com/kr/text v0.1.0
github.com/pkg/errors v0.9.1
github.com/pmezard/go-difflib v1.0.0
github.com/slimsag/godocmd v0.0.0-20161025000126-a1005ad29fe3
github.com/sourcegraph/lsif-go/internal/testdata v0.0.0-20200804185623-bb090d50c787
github.com/stretchr/objx v0.1.0
github.com/stretchr/testify v1.4.0
golang.org/x/crypto v0.0.0-20191011191535-87dc89f01550
golang.org/x/mod v0.1.1-0.20191105210325-c90efee705ee
golang.org/x/net v0.0.0-20190620200207-3b0461eec859
golang.org/x/sync v0.0.0-20190423024810-112230192c58
golang.org/x/sys v0.0.0-20190428183149-804c0c7841b5
golang.org/x/text v0.3.2
golang.org/x/tools v0.0.0-20200212150539-ea181f53ac56
golang.org/x/xerrors v0.0.0-20191204190536-9bdfabe68543
gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15
gopkg.in/yaml.v2 v2.2.5
`

func TestParseGoListOutput(t *testing.T) {
	module, dependencies := parseGoListOutput(strings.TrimSpace(testGoListOutput))
	if module != "github.com/sourcegraph/lsif-go" {
		t.Errorf("unexpected module. want=%s have=%s", "github.com/sourcegraph/lsif-go", module)
	}

	expectedDependencies := map[string]string{
		"github.com/alecthomas/kingpin":                    "v2.2.6+incompatible",
		"github.com/alecthomas/template":                   "v0.0.0-fb15b899a751",
		"github.com/alecthomas/units":                      "v0.0.0-f65c72e2690d",
		"github.com/davecgh/go-spew":                       "v1.1.1",
		"github.com/efritz/pentimento":                     "v0.0.0-ade47d831101",
		"github.com/google/go-cmp":                         "v0.5.1",
		"github.com/kr/pretty":                             "v0.1.0",
		"github.com/kr/pty":                                "v1.1.1",
		"github.com/kr/text":                               "v0.1.0",
		"github.com/pkg/errors":                            "v0.9.1",
		"github.com/pmezard/go-difflib":                    "v1.0.0",
		"github.com/slimsag/godocmd":                       "v0.0.0-a1005ad29fe3",
		"github.com/sourcegraph/lsif-go/internal/testdata": "v0.0.0-bb090d50c787",
		"github.com/stretchr/objx":                         "v0.1.0",
		"github.com/stretchr/testify":                      "v1.4.0",
		"golang.org/x/crypto":                              "v0.0.0-87dc89f01550",
		"golang.org/x/mod":                                 "v0.1.1-0.20191105210325-c90efee705ee",
		"golang.org/x/net":                                 "v0.0.0-3b0461eec859",
		"golang.org/x/sync":                                "v0.0.0-112230192c58",
		"golang.org/x/sys":                                 "v0.0.0-804c0c7841b5",
		"golang.org/x/text":                                "v0.3.2",
		"golang.org/x/tools":                               "v0.0.0-ea181f53ac56",
		"golang.org/x/xerrors":                             "v0.0.0-9bdfabe68543",
		"gopkg.in/check.v1":                                "v1.0.0-41f04d3bba15",
		"gopkg.in/yaml.v2":                                 "v2.2.5",
	}
	if diff := cmp.Diff(expectedDependencies, dependencies); diff != "" {
		t.Errorf("unexpected dependencies (-want +got): %s", diff)
	}
}
