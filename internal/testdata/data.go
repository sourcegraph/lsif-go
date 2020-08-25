package testdata

import (
	"context"

	"github.com/sourcegraph/lsif-go/internal/testdata/internal/secret"
)

// TestInterface is an interface used for testing.
type TestInterface interface {
	// Do does a test thing.
	Do(ctx context.Context, data string) (score int, _ error)
}

type (
	// TestStruct is a struct used for testing.
	TestStruct struct {
		// SimpleA docs
		SimpleA int
		// SimpleB docs
		SimpleB int
		// SimpleC docs
		SimpleC int

		FieldWithTag           string `json:"tag"`
		FieldWithAnonymousType struct {
			NestedA string
			NestedB string
			// NestedC docs
			NestedC string
		}

		EmptyStructField struct{}
	}

	TestEmptyStruct struct{}
)

// Score is just a hardcoded number.
const Score = uint64(42)
const secretScore = secret.SecretScore

// Doer is similar to the test interface (but not the same).
func (ts *TestStruct) Doer(ctx context.Context, data string) (score int, err error) {
	return Score, nil
}
