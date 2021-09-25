package testdata

import (
	"context"

	"github.com/sourcegraph/lsif-go/internal/testdata/fixtures/internal/secret"
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

const SomeString = "foobar"
const LongString = "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed tincidunt viverra aliquam. Phasellus finibus, arcu eu commodo porta, dui quam dictum ante, nec porta enim leo quis felis. Pellentesque habitant morbi tristique senectus et netus et malesuada fames ac turpis egestas. Curabitur luctus orci tortor, non condimentum arcu bibendum ut. Proin sit amet vulputate lorem, ut egestas arcu. Curabitur quis sagittis mi. Aenean elit sem, imperdiet ut risus eget, varius varius erat.\nNullam lobortis tortor sed sodales consectetur. Aenean condimentum vehicula elit, eget interdum ante finibus nec. Mauris mollis, nulla eu vehicula rhoncus, eros lectus viverra tellus, ac hendrerit quam massa et felis. Nunc vestibulum diam a facilisis sollicitudin. Aenean nec varius metus. Sed nec diam nibh. Ut erat erat, suscipit et ante eget, tincidunt condimentum orci. Aenean nec facilisis augue, ac sodales ex. Nulla dictum hendrerit tempus. Aliquam fringilla tortor in massa molestie, quis bibendum nulla ullamcorper. Suspendisse congue laoreet elit, vitae consectetur orci facilisis non. Aliquam tempus ultricies sapien, rhoncus tincidunt nisl tincidunt eget. Aliquam nisi ante, rutrum eget viverra imperdiet, congue ut nunc. Donec mollis sed tellus vel placerat. Sed mi ex, fringilla a fermentum a, tincidunt eget lectus.\nPellentesque lacus nibh, accumsan eget feugiat nec, gravida eget urna. Donec quam velit, imperdiet in consequat eget, ultricies eget nunc. Curabitur interdum vel sem et euismod. Donec sed vulputate odio, sit amet bibendum tellus. Integer pellentesque nunc eu turpis cursus, vestibulum sodales ipsum posuere. Vestibulum ante ipsum primis in faucibus orci luctus et ultrices posuere cubilia curae; Ut at vestibulum sapien. In hac habitasse platea dictumst. Nullam sed lobortis urna, non bibendum ipsum. Sed in sapien quis purus semper fringilla. Integer ut egestas nulla, eu ornare lectus. Maecenas quis sapien condimentum, dignissim urna quis, hendrerit neque. Donec cursus sit amet metus eu mollis.\nSed scelerisque vitae odio non egestas. Cras hendrerit tortor mauris. Aenean quis imperdiet nulla, a viverra purus. Orci varius natoque penatibus et magnis dis parturient montes, nascetur ridiculus mus. Praesent finibus faucibus orci, sed ultrices justo iaculis ut. Ut libero massa, condimentum at elit non, fringilla iaculis quam. Sed sit amet ipsum placerat, tincidunt sem in, efficitur lacus. Curabitur ligula orci, tempus ut magna eget, sodales tristique odio.\nPellentesque in libero ac risus pretium ultrices. In hac habitasse platea dictumst. Curabitur a quam sed orci tempus luctus. Integer commodo nec odio quis consequat. Aenean vitae dapibus augue, nec dictum lectus. Etiam sit amet leo diam. Duis eu ligula venenatis, fermentum lacus vel, interdum odio. Vivamus sit amet libero vitae elit interdum cursus et eu erat. Cras interdum augue sit amet ex aliquet tempor. Praesent dolor nisl, convallis bibendum mauris a, euismod commodo ante. Phasellus non ipsum condimentum, molestie dolor quis, pretium nisi. Mauris augue urna, fermentum ut lacinia a, efficitur vitae odio. Praesent finibus nisl et dolor luctus faucibus. Donec eget lectus sed mi porttitor placerat ac eu odio."
const ConstMath = 1 + (2+3)*5

type StringAlias string

const AliasedString StringAlias = "foobar"

// Doer is similar to the test interface (but not the same).
func (ts *TestStruct) Doer(ctx context.Context, data string) (score int, err error) {
	return Score, nil
}

// StructTagRegression is a struct that caused panic in the wild. Added here to
// support a regression test.
//
// See https://github.com/tal-tech/go-zero/blob/11dd3d75ecceaa3f5772024fb3f26dec1ada8e9c/core/mapping/unmarshaler_test.go#L2272.
type StructTagRegression struct {
	Value int `key:",range=[:}"`
}

type TestEqualsStruct = struct {
	Value int
}

type ShellStruct struct {
	// Ensure this field comes before the definition
	// so that we grab the correct one in our unit
	// tests.
	InnerStruct
}

type InnerStruct struct{}
