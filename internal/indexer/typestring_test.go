package indexer

import (
	"go/types"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestTypeStringBasic(t *testing.T) {
	_, f := findDefinitionByName(t, getTestPackages(t), "Score")

	if signature, _ := typeString(f); signature != "const Score uint64" {
		t.Errorf("unexpected type string. want=%q have=%q", "const Score uint64", signature)
	}
}

func TestTypeStringPackage(t *testing.T) {
	p := types.NewPkgName(42, nil, "sync", nil)

	if signature, _ := typeString(p); signature != "package sync" {
		t.Errorf("unexpected type string. want=%q have=%q", "package sync", signature)
	}
}

func TestTypeStringFunction(t *testing.T) {
	_, f := findDefinitionByName(t, getTestPackages(t), "Parallel")

	if signature, _ := typeString(f); signature != "func Parallel(ctx Context, fns ...ParallelizableFunc) error" {
		t.Errorf("unexpected type string. want=%q have=%q", "func Parallel(ctx Context, fns ...ParallelizableFunc) error", signature)
	}
}

func TestTypeStringInterface(t *testing.T) {
	_, f := findDefinitionByName(t, getTestPackages(t), "TestInterface")

	signature, extra := typeString(f)
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

	signature, extra := typeString(f)
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

	signature, extra := typeString(f)
	if signature != "type TestEmptyStruct struct" {
		t.Errorf("unexpected type string. want=%q have=%q", "type TestEmptyStruct struct", signature)
	}

	expectedExtra := `struct{}`
	if diff := cmp.Diff(expectedExtra, extra); diff != "" {
		t.Errorf("unexpected extra (-want +got): %s", diff)
	}
}

func TestStructTagRegression(t *testing.T) {
	_, f := findDefinitionByName(t, getTestPackages(t), "StructTagRegression")

	signature, extra := typeString(f)
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
