package indexer

import (
	"go/types"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestTypeStringPackage(t *testing.T) {
	p := types.NewPkgName(42, nil, "sync", nil)

	if signature, _ := TypeStringForObject(p); signature != "package sync" {
		t.Errorf("unexpected type string. want=%q have=%q", "package sync", signature)
	}
}

func TestTypeStringFunction(t *testing.T) {
	_, f := findDefinitionByName(t, getTestPackages(t), "Parallel")

	if signature, _ := TypeStringForObject(f); signature != "func Parallel(ctx Context, fns ...ParallelizableFunc) error" {
		t.Errorf("unexpected type string. want=%q have=%q", "func Parallel(ctx Context, fns ...ParallelizableFunc) error", signature)
	}
}

func TestTypeStringInterface(t *testing.T) {
	_, f := findDefinitionByName(t, getTestPackages(t), "TestInterface")

	signature, extra := TypeStringForObject(f)
	if signature != "type TestInterface interface" {
		t.Errorf("unexpected type string. want=%q have=%q", "type TestInterface interface", signature)
	}

	expectedExtra := strings.TrimSpace(stripIndent(`
		interface {
		    Do(ctx Context, data string) (score int, _ error)
		}
	`))
	if diff := cmp.Diff(expectedExtra, extra); diff != "" {
		t.Errorf("unexpected extra (-want +got): %s", diff)
	}
}

func TestTypeStringStruct(t *testing.T) {
	_, f := findDefinitionByName(t, getTestPackages(t), "TestStruct")

	signature, extra := TypeStringForObject(f)
	if signature != "type TestStruct struct" {
		t.Errorf("unexpected type string. want=%q have=%q", "type TestStruct struct", signature)
	}

	expectedExtra := strings.TrimSpace(stripIndent(`
		struct {
		    SimpleA int
		    SimpleB int
		    SimpleC int
		    FieldWithTag string "json:\"tag\""
		    FieldWithAnonymousType struct {
		        NestedA string
		        NestedB string
		        NestedC string
		    }
		    EmptyStructField struct{}
		}
	`))
	if diff := cmp.Diff(expectedExtra, extra); diff != "" {
		t.Errorf("unexpected extra (-want +got): %s", diff)
	}
}

func TestTypeStringEmptyStruct(t *testing.T) {
	_, f := findDefinitionByName(t, getTestPackages(t), "TestEmptyStruct")

	signature, extra := TypeStringForObject(f)
	if signature != "type TestEmptyStruct struct" {
		t.Errorf("unexpected type string. want=%q have=%q", "type TestEmptyStruct struct", signature)
	}

	expectedExtra := `struct{}`
	if diff := cmp.Diff(expectedExtra, extra); diff != "" {
		t.Errorf("unexpected extra (-want +got): %s", diff)
	}
}

func TestTypeStringNameEqualsAnonymousStruct(t *testing.T) {
	_, f := findDefinitionByName(t, getTestPackages(t), "TestEqualsStruct")

	signature, extra := TypeStringForObject(f)
	if signature != "type TestEqualsStruct = struct" {
		t.Errorf("unexpected type string. want=%q have=%q", "type TestEqualsStruct = struct", signature)
	}

	expectedExtra := strings.TrimSpace(stripIndent(`
		struct {
		    Value int
		}
	`))
	if diff := cmp.Diff(expectedExtra, extra); diff != "" {
		t.Errorf("unexpected extra (-want +got): %s", diff)
	}
}

func TestStructTagRegression(t *testing.T) {
	_, f := findDefinitionByName(t, getTestPackages(t), "StructTagRegression")

	signature, extra := TypeStringForObject(f)
	if signature != "type StructTagRegression struct" {
		t.Errorf("unexpected type string. want=%q have=%q", "type StructTagRegression struct", signature)
	}

	expectedExtra := strings.TrimSpace(stripIndent(`
		struct {
		    Value int "key:\",range=[:}\""
		}
	`))

	if diff := cmp.Diff(expectedExtra, extra); diff != "" {
		t.Errorf("unexpected extra (-want +got): %s", diff)
	}
}

func TestTypeStringConstNumber(t *testing.T) {
	_, obj := findDefinitionByName(t, getTestPackages(t), "Score")

	signature, _ := TypeStringForObject(obj)
	if signature != "const Score uint64 = 42" {
		t.Errorf("unexpected type string. want=%q have=%q", "const Score uint64 = 42", signature)
	}
}

func TestTypeStringConstString(t *testing.T) {
	_, obj := findDefinitionByName(t, getTestPackages(t), "SomeString")

	signature, _ := TypeStringForObject(obj)
	if signature != `const SomeString untyped string = "foobar"` {
		t.Errorf("unexpected type string. want=%q have=%q", `const SomeString string = "foobar"`, signature)
	}
}

func TestTypeStringConstTruncatedString(t *testing.T) {
	_, obj := findDefinitionByName(t, getTestPackages(t), "LongString")

	signature, _ := TypeStringForObject(obj)
	if signature != `const LongString untyped string = "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed tincidu...` {
		t.Errorf("unexpected type string. want=%q have=%q", `const LongString untyped string = "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed tincidu...`, signature)
	}
}

func TestTypeStringConstArithmetic(t *testing.T) {
	_, obj := findDefinitionByName(t, getTestPackages(t), "ConstMath")

	signature, _ := TypeStringForObject(obj)
	if signature != `const ConstMath untyped int = 26` {
		t.Errorf("unexpected type string. want=%q have=%q", `const ConstMath untyped int = 26`, signature)
	}
}

func TestTypeStringAliasedString(t *testing.T) {
	_, obj := findDefinitionByName(t, getTestPackages(t), "AliasedString")

	signature, _ := TypeStringForObject(obj)
	if signature != `const AliasedString StringAlias = "foobar"` {
		t.Errorf("unexpected type string. want=%q have=%q", `const AliasedString StringAlias = "foobar"`, signature)
	}
}
