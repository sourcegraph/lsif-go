package testdata

import "github.com/sourcegraph/lsif-go/internal/testdata/internal/secret"

func test() {
	secretType := secret.SecretType{}
	println(secretType)
}
