// Package testdata
package testdata

// Const is a constant equal to 5. It's the best constant I've ever written. 😹
const Const = 5

// Docs for the const block itself.
const (
	// ConstBlock1 is a constant in a block.
	ConstBlock1 = 1

	// ConstBlock2 is a constant in a block.
	ConstBlock2 = 2
)

// Var is a variable interface.
var Var Interface = &Struct{Field: "bar!"}

// unexportedVar is an unexported variable interface.
var unexportedVar Interface = &Struct{Field: "bar!"}

var BigVar Interface = &Struct{
	Field: "bar!",
	Anonymous: struct {
		FieldA int
		FieldB int
		FieldC int
	}{FieldA: 1337},
}

// What are docs, really?
// I can't say for sure, I don't write any.
// But look, a CAT!
//
// 	      |\      _,,,---,,_
// 	ZZZzz /,`.-'`'    -.  ;-;;,_
// 	     |,4-  ) )-,_. ,\ (  `'-'
// 	    '---''(_/--'  `-'\_)
//
// It's sleeping! Some people write that as `sleeping` but Markdown
// isn't allowed in Go docstrings, right? right?!
var(
	// This has some docs
	VarBlock1 = "if you're reading this"

	VarBlock2 = "hi"
)

// Embedded is a struct, to be embedded in another struct.
type Embedded struct {
	// EmbeddedField has some docs!
	EmbeddedField string
	Field string // conflicts with parent "Field"
}

type Struct struct {
	*Embedded
	Field string
	Anonymous struct {
		FieldA int
		FieldB int
		FieldC int
	}
}}

// StructMethod has some docs!
func (s *Struct) StructMethod() {}

func (s *Struct) ImplementsInterface() string { return "hi!" }

func (s *Struct) MachineLearning(
	param1 float32, // It's ML, I can't describe what this param is.

	// We call the below hyperparameters because, uhh, well:
	//
	// 	  ,-.       _,---._ __  / \
	// 	 /  )    .-'       `./ /   \
	// 	 (  (   ,'            `/    /|
	// 	  \  `-"             \'\   / |
	// 	   `.              ,  \ \ /  |
	// 		/`.          ,'-`----Y   |
	// 	   (            ;        |   '
	// 	   |  ,-.    ,-'         |  /
	// 	   |  | (   |        hjw | /
	// 	   )  |  \  `.___________|/
	// 	   `--'   `--'
	//
	hyperparam2 float32,
	hyperparam3 float32
) float32 {
	return param1+(hyperparam2**hyperparam3) // lol is this all ML is? I'm gonna be rich
}

// Interface has docs too
type Interface interface {
	ImplementsInterface() string
}

func NewInterface() Interface { return nil }

// Yeah this is some Go magic incantation which is common.
//
// 	 ,_     _
// 	 |\\_,-~/
// 	 / _  _ |    ,--.
// 	(  @  @ )   / ,-'
// 	 \  _T_/-._( (
// 	/         `. \
// 	|         _  \ |
// 	\ \ ,  /      |
// 	 || |-_\__   /
// 	((_/`(____,-'
//
var _ = Interface(&Struct{})

// Go can be fun
type (
	// And confusing
	X struct {
		bar string
	}

	Y struct {
		baz float
	}
)
