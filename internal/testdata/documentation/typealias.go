package testdata

import (
	"github.com/sourcegraph/lsif-go/internal/testdata/documentation/internal/secret"
)

// Type aliased doc
type SecretBurger = secret.Burger

type BadBurger = struct {
	Field string
}
