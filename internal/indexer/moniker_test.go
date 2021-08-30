package indexer

import (
	"go/constant"
	"go/token"
	"go/types"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/sourcegraph/lsif-go/internal/gomod"
	"github.com/sourcegraph/sourcegraph/lib/codeintel/lsif/protocol/writer"
)

func TestEmitExportMoniker(t *testing.T) {
	w := &capturingWriter{}

	indexer := &Indexer{
		repositoryRemote:      "github.com/sourcegraph/lsif-go",
		repositoryRoot:        "/users/efritz/dev/sourcegraph/lsif-go",
		projectRoot:           "/users/efritz/dev/sourcegraph/lsif-go",
		moduleName:            "https://github.com/sourcegraph/lsif-go",
		moduleVersion:         "3.14.159",
		emitter:               writer.NewEmitter(w),
		importMonikerIDs:      map[string]uint64{},
		packageInformationIDs: map[string]uint64{},
		stripedMutex:          newStripedMutex(),
	}

	object := types.NewConst(
		token.Pos(42),
		types.NewPackage("github.com/test/pkg", "pkg"),
		"foobar",
		&types.Basic{},
		constant.MakeBool(true),
	)

	indexer.emitExportMoniker(123, nil, object)

	monikers := findMonikersByRangeOrReferenceResultID(w, 123)
	if monikers == nil || len(monikers) < 1 {
		t.Fatalf("could not find moniker")
	}
	if monikers[0].Kind != "export" {
		t.Errorf("incorrect moniker kind. want=%q have=%q", "export", monikers[0].Kind)
	}
	if monikers[0].Scheme != "gomod" {
		t.Errorf("incorrect moniker scheme want=%q have=%q", "gomod", monikers[0].Scheme)
	}
	if monikers[0].Identifier != "github.com/test/pkg:foobar" {
		t.Errorf("incorrect moniker identifier. want=%q have=%q", "github.com/test/pkg:foobar", monikers[0].Identifier)
	}

	packageInformation := findPackageInformationByMonikerID(w, monikers[0].ID)
	if monikers == nil || len(monikers) < 1 {
		t.Fatalf("could not find package information")
	}
	if packageInformation[0].Name != "https://github.com/sourcegraph/lsif-go" {
		t.Errorf("incorrect moniker name. want=%q have=%q", "https://github.com/sourcegraph/lsif-go", monikers[0].Kind)
	}
	if packageInformation[0].Version != "3.14.159" {
		t.Errorf("incorrect moniker scheme want=%q have=%q", "3.14.159", monikers[0].Scheme)
	}
}

func TestEmitExportMonikerPreGoMod(t *testing.T) {
	w := &capturingWriter{}

	indexer := &Indexer{
		repositoryRemote:      "github.com/sourcegraph/lsif-go",
		repositoryRoot:        "/users/efritz/dev/sourcegraph/lsif-go",
		projectRoot:           "/users/efritz/dev/sourcegraph/lsif-go",
		moduleName:            "https://github.com/sourcegraph/lsif-go",
		moduleVersion:         "3.14.159",
		emitter:               writer.NewEmitter(w),
		importMonikerIDs:      map[string]uint64{},
		packageInformationIDs: map[string]uint64{},
		stripedMutex:          newStripedMutex(),
	}

	object := types.NewConst(
		token.Pos(42),
		types.NewPackage("_/users/efritz/dev/sourcegraph/lsif-go/internal/git", "pkg"),
		"InferRemote",
		&types.Basic{},
		constant.MakeBool(true),
	)

	indexer.emitExportMoniker(123, nil, object)

	monikers := findMonikersByRangeOrReferenceResultID(w, 123)
	if monikers == nil || len(monikers) < 1 {
		t.Fatalf("could not find moniker")
	}
	if monikers[0].Kind != "export" {
		t.Errorf("incorrect moniker kind. want=%q have=%q", "export", monikers[0].Kind)
	}
	if monikers[0].Scheme != "gomod" {
		t.Errorf("incorrect moniker scheme want=%q have=%q", "gomod", monikers[0].Scheme)
	}
	if monikers[0].Identifier != "github.com/sourcegraph/lsif-go/internal/git:InferRemote" {
		t.Errorf("incorrect moniker identifier. want=%q have=%q", "github.com/sourcegraph/lsif-go/internal/git:InferRemote", monikers[0].Identifier)
	}

	packageInformation := findPackageInformationByMonikerID(w, monikers[0].ID)
	if monikers == nil || len(monikers) < 1 {
		t.Fatalf("could not find package information")
	}
	if packageInformation[0].Name != "https://github.com/sourcegraph/lsif-go" {
		t.Errorf("incorrect moniker kind. want=%q have=%q", "https://github.com/sourcegraph/lsif-go", monikers[0].Kind)
	}
	if packageInformation[0].Version != "3.14.159" {
		t.Errorf("incorrect moniker scheme want=%q have=%q", "3.14.159", monikers[0].Scheme)
	}
}

func TestEmitImportMoniker(t *testing.T) {
	w := &capturingWriter{}

	indexer := &Indexer{
		dependencies: map[string]gomod.GoModule{
			"github.com/test/pkg/sub1": {Name: "github.com/test/pkg/sub1", Version: "1.2.3-deadbeef"},
		},
		emitter:               writer.NewEmitter(w),
		importMonikerIDs:      map[string]uint64{},
		packageInformationIDs: map[string]uint64{},
		stripedMutex:          newStripedMutex(),
	}

	object := types.NewConst(
		token.Pos(42),
		types.NewPackage("github.com/test/pkg/sub1/sub2/sub3", "sub3"),
		"foobar",
		&types.Basic{},
		constant.MakeBool(true),
	)

	indexer.emitImportMoniker(123, nil, object)

	monikers := findMonikersByRangeOrReferenceResultID(w, 123)
	if monikers == nil || len(monikers) < 1 {
		t.Fatalf("could not find moniker")
	}
	if monikers[0].Kind != "import" {
		t.Errorf("incorrect moniker kind. want=%q have=%q", "import", monikers[0].Kind)
	}
	if monikers[0].Scheme != "gomod" {
		t.Errorf("incorrect moniker scheme want=%q have=%q", "gomod", monikers[0].Scheme)
	}
	if monikers[0].Identifier != "github.com/test/pkg/sub1/sub2/sub3:foobar" {
		t.Errorf("incorrect moniker identifier. want=%q have=%q", "github.com/test/pkg/sub1/sub2/sub3:foobar", monikers[0].Identifier)
	}

	packageInformation := findPackageInformationByMonikerID(w, monikers[0].ID)
	if monikers == nil || len(monikers) < 1 {
		t.Fatalf("could not find package information")
	}
	if packageInformation[0].Name != "github.com/test/pkg/sub1" {
		t.Errorf("incorrect moniker kind. want=%q have=%q", "github.com/test/pkg/sub1", monikers[0].Kind)
	}
	if packageInformation[0].Version != "1.2.3-deadbeef" {
		t.Errorf("incorrect moniker scheme want=%q have=%q", "1.2.3-deadbeef", monikers[0].Scheme)
	}
}

func TestPackagePrefixes(t *testing.T) {
	expectedPackages := []string{
		"github.com/foo/bar/baz/bonk/internal/secrets",
		"github.com/foo/bar/baz/bonk/internal",
		"github.com/foo/bar/baz/bonk",
		"github.com/foo/bar/baz",
		"github.com/foo/bar",
		"github.com/foo",
		"github.com",
	}

	if diff := cmp.Diff(expectedPackages, packagePrefixes("github.com/foo/bar/baz/bonk/internal/secrets")); diff != "" {
		t.Errorf("unexpected package prefixes (-want +got): %s", diff)
	}
}

func TestMonikerIdentifierBasic(t *testing.T) {
	packages := getTestPackages(t)
	p, obj := findUseByName(t, packages, "Score")

	if identifier := makeMonikerIdentifier(NewPackageDataCache(), p, obj); identifier != "Score" {
		t.Errorf("unexpected moniker identifier. want=%q have=%q", "Score", identifier)
	}
}

func TestMonikerIdentifierPackageName(t *testing.T) {
	packages := getTestPackages(t)
	p, obj := findUseByName(t, packages, "sync")

	if identifier := makeMonikerIdentifier(NewPackageDataCache(), p, obj); identifier != "" {
		t.Errorf("unexpected moniker identifier. want=%q have=%q", "", identifier)
	}
}

func TestMonikerIdentifierSignature(t *testing.T) {
	packages := getTestPackages(t)
	p, obj := findDefinitionByName(t, packages, "Doer")

	if identifier := makeMonikerIdentifier(NewPackageDataCache(), p, obj); identifier != "TestStruct.Doer" {
		t.Errorf("unexpected moniker identifier. want=%q have=%q", "TestStruct.Doer", identifier)
	}
}

func TestMonikerIdentifierField(t *testing.T) {
	packages := getTestPackages(t)
	p, obj := findDefinitionByName(t, packages, "NestedB")

	if identifier := makeMonikerIdentifier(NewPackageDataCache(), p, obj); identifier != "TestStruct.FieldWithAnonymousType.NestedB" {
		t.Errorf("unexpected moniker identifier. want=%q have=%q", "TestStruct.FieldWithAnonymousType.NestedB", identifier)
	}
}

func TestMonikerEmbeddedField(t *testing.T) {
	packages := getTestPackages(t)
	p, obj := findDefinitionByName(t, packages, "InnerStruct")

	if identifier := makeMonikerIdentifier(NewPackageDataCache(), p, obj); identifier != "ShellStruct.InnerStruct" {
		t.Errorf("unexpected moniker identifier. want=%q have=%q", "ShellStruct.InnerStruct", identifier)
	}
}

func TestJoinMonikerParts(t *testing.T) {
	testCases := []struct {
		input    []string
		expected string
	}{
		{input: []string{}, expected: ""},
		{input: []string{"a"}, expected: "a"},
		{input: []string{"a", "", "c"}, expected: "a:c"},
		{input: []string{"a", "b", "c"}, expected: "a:b:c"},
	}

	for _, testCase := range testCases {
		if actual := joinMonikerParts(testCase.input...); actual != testCase.expected {
			t.Errorf("unexpected moniker identifier. want=%q have=%q", testCase.expected, actual)
		}
	}
}
