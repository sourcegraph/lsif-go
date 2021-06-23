# Package testdata

Package testdata 

testdata is a small package containing sample Go source code used for testing the indexing routines of github.com/sourcegraph/lsif-go. 

## Index

* Subpages
  * [internal](internal.md)
  * [duplicate_path_id](duplicate_path_id.md)
* [Constants](#const)
    * [const Const](#Const)
    * [const ConstBlock1](#ConstBlock1)
    * [const ConstBlock2](#ConstBlock2)
    * [const Score](#Score)
    * [const secretScore](#secretScore)
    * [const SomeString](#SomeString)
    * [const LongString](#LongString)
    * [const ConstMath](#ConstMath)
    * [const AliasedString](#AliasedString)
* [Variables](#var)
    * [var Var](#Var)
    * [var unexportedVar](#unexportedVar)
    * [var x](#x)
    * [var BigVar](#BigVar)
    * [var VarBlock1](#VarBlock1)
    * [var VarBlock2](#VarBlock2)
* [Types](#type)
    * [type Embedded struct](#Embedded)
    * [type Struct struct](#Struct)
        * [func (s *Struct) StructMethod()](#Struct.StructMethod)
        * [func (s *Struct) ImplementsInterface() string](#Struct.ImplementsInterface)
        * [func (s *Struct) MachineLearning(param1 float32,...](#Struct.MachineLearning)
    * [type Interface interface](#Interface)
        * [func NewInterface() Interface](#NewInterface)
    * [type X struct](#X)
    * [type Y struct](#Y)
    * [type Inner struct](#Inner)
    * [type Outer struct](#Outer)
    * [type TestInterface interface](#TestInterface)
    * [type TestStruct struct](#TestStruct)
        * [func (ts *TestStruct) Doer(ctx context.Context, data string) (score int, err error)](#TestStruct.Doer)
    * [type TestEmptyStruct struct{}](#TestEmptyStruct)
    * [type StringAlias string](#StringAlias)
    * [type StructTagRegression struct](#StructTagRegression)
    * [type TestEqualsStruct struct](#TestEqualsStruct)
    * [type ShellStruct struct](#ShellStruct)
    * [type InnerStruct struct{}](#InnerStruct)
    * [type ParallelizableFunc func(ctx context.Context) error](#ParallelizableFunc)
    * [type SecretBurger secret.Burger](#SecretBurger)
    * [type BadBurger struct](#BadBurger)
* [Functions](#func)
    * [func useOfCompositeStructs()](#useOfCompositeStructs)
    * [func Parallel(ctx context.Context, fns ...ParallelizableFunc) error](#Parallel)
    * [func Switch(interfaceValue interface{}) bool](#Switch)


## <a id="const" href="#const">Constants</a>

```
tags: [private]
```

### <a id="Const" href="#Const">const Const</a>

```
searchKey: testdata.Const
```

```Go
const Const = 5
```

Const is a constant equal to 5. It's the best constant I've ever written. 😹 

### <a id="ConstBlock1" href="#ConstBlock1">const ConstBlock1</a>

```
searchKey: testdata.ConstBlock1
```

```Go
const ConstBlock1 = 1
```

Docs for the const block itself. 

ConstBlock1 is a constant in a block. 

### <a id="ConstBlock2" href="#ConstBlock2">const ConstBlock2</a>

```
searchKey: testdata.ConstBlock2
```

```Go
const ConstBlock2 = 2
```

Docs for the const block itself. 

ConstBlock2 is a constant in a block. 

### <a id="Score" href="#Score">const Score</a>

```
searchKey: testdata.Score
```

```Go
const Score = uint64(42)
```

Score is just a hardcoded number. 

### <a id="secretScore" href="#secretScore">const secretScore</a>

```
searchKey: testdata.secretScore
tags: [private]
```

```Go
const secretScore = secret.SecretScore
```

### <a id="SomeString" href="#SomeString">const SomeString</a>

```
searchKey: testdata.SomeString
```

```Go
const SomeString = "foobar"
```

### <a id="LongString" href="#LongString">const LongString</a>

```
searchKey: testdata.LongString
```

```Go
const LongString = ...
```

### <a id="ConstMath" href="#ConstMath">const ConstMath</a>

```
searchKey: testdata.ConstMath
```

```Go
const ConstMath = 1 + (2+3)*5
```

### <a id="AliasedString" href="#AliasedString">const AliasedString</a>

```
searchKey: testdata.AliasedString
```

```Go
const AliasedString StringAlias = "foobar"
```

## <a id="var" href="#var">Variables</a>

```
tags: [private]
```

### <a id="Var" href="#Var">var Var</a>

```
searchKey: testdata.Var
```

```Go
var Var Interface = &Struct{Field: "bar!"}
```

Var is a variable interface. 

### <a id="unexportedVar" href="#unexportedVar">var unexportedVar</a>

```
searchKey: testdata.unexportedVar
tags: [private]
```

```Go
var unexportedVar Interface = &Struct{Field: "bar!"}
```

unexportedVar is an unexported variable interface. 

### <a id="x" href="#x">var x</a>

```
searchKey: testdata.x
tags: [private]
```

```Go
var x error
```

x has a builtin error type 

### <a id="BigVar" href="#BigVar">var BigVar</a>

```
searchKey: testdata.BigVar
```

```Go
var BigVar Interface = ...
```

### <a id="VarBlock1" href="#VarBlock1">var VarBlock1</a>

```
searchKey: testdata.VarBlock1
```

```Go
var VarBlock1 = "if you're reading this"
```

What are docs, really? I can't say for sure, I don't write any. But look, a CAT! 

```
      |\      _,,,---,,_
ZZZzz /,`.-'`'    -.  ;-;;,_
     |,4-  ) )-,_. ,\ (  `'-'
    '---''(_/--'  `-'\_)

```
It's sleeping! Some people write that as `sleeping` but Markdown isn't allowed in Go docstrings, right? right?! 

This has some docs 

### <a id="VarBlock2" href="#VarBlock2">var VarBlock2</a>

```
searchKey: testdata.VarBlock2
```

```Go
var VarBlock2 = "hi"
```

What are docs, really? I can't say for sure, I don't write any. But look, a CAT! 

```
      |\      _,,,---,,_
ZZZzz /,`.-'`'    -.  ;-;;,_
     |,4-  ) )-,_. ,\ (  `'-'
    '---''(_/--'  `-'\_)

```
It's sleeping! Some people write that as `sleeping` but Markdown isn't allowed in Go docstrings, right? right?! 

## <a id="type" href="#type">Types</a>

```
tags: [private]
```

### <a id="Embedded" href="#Embedded">type Embedded struct</a>

```
searchKey: testdata.Embedded
```

```Go
type Embedded struct {
	// EmbeddedField has some docs!
	EmbeddedField string
	Field         string // conflicts with parent "Field"
}
```

Embedded is a struct, to be embedded in another struct. 

### <a id="Struct" href="#Struct">type Struct struct</a>

```
searchKey: testdata.Struct
```

```Go
type Struct struct {
	*Embedded
	Field     string
	Anonymous struct {
		FieldA int
		FieldB int
		FieldC int
	}
}
```

#### <a id="Struct.StructMethod" href="#Struct.StructMethod">func (s *Struct) StructMethod()</a>

```
searchKey: testdata.Struct.StructMethod
```

```Go
func (s *Struct) StructMethod()
```

StructMethod has some docs! 

#### <a id="Struct.ImplementsInterface" href="#Struct.ImplementsInterface">func (s *Struct) ImplementsInterface() string</a>

```
searchKey: testdata.Struct.ImplementsInterface
```

```Go
func (s *Struct) ImplementsInterface() string
```

#### <a id="Struct.MachineLearning" href="#Struct.MachineLearning">func (s *Struct) MachineLearning(param1 float32,...</a>

```
searchKey: testdata.Struct.MachineLearning
```

```Go
func (s *Struct) MachineLearning(
	param1 float32,

	hyperparam2 float32,
	hyperparam3 float32,
) float32
```

### <a id="Interface" href="#Interface">type Interface interface</a>

```
searchKey: testdata.Interface
```

```Go
type Interface interface {
	ImplementsInterface() string
}
```

Interface has docs too 

#### <a id="NewInterface" href="#NewInterface">func NewInterface() Interface</a>

```
searchKey: testdata.NewInterface
```

```Go
func NewInterface() Interface
```

### <a id="X" href="#X">type X struct</a>

```
searchKey: testdata.X
```

```Go
type X struct {
	bar string
}
```

Go can be fun 

And confusing 

### <a id="Y" href="#Y">type Y struct</a>

```
searchKey: testdata.Y
```

```Go
type Y struct {
	baz float
}
```

Go can be fun 

### <a id="Inner" href="#Inner">type Inner struct</a>

```
searchKey: testdata.Inner
```

```Go
type Inner struct {
	X int
	Y int
	Z int
}
```

### <a id="Outer" href="#Outer">type Outer struct</a>

```
searchKey: testdata.Outer
```

```Go
type Outer struct {
	Inner
	W int
}
```

### <a id="TestInterface" href="#TestInterface">type TestInterface interface</a>

```
searchKey: testdata.TestInterface
```

```Go
type TestInterface interface {
	// Do does a test thing.
	Do(ctx context.Context, data string) (score int, _ error)
}
```

TestInterface is an interface used for testing. 

### <a id="TestStruct" href="#TestStruct">type TestStruct struct</a>

```
searchKey: testdata.TestStruct
```

```Go
type TestStruct struct {
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
```

TestStruct is a struct used for testing. 

#### <a id="TestStruct.Doer" href="#TestStruct.Doer">func (ts *TestStruct) Doer(ctx context.Context, data string) (score int, err error)</a>

```
searchKey: testdata.TestStruct.Doer
```

```Go
func (ts *TestStruct) Doer(ctx context.Context, data string) (score int, err error)
```

Doer is similar to the test interface (but not the same). 

### <a id="TestEmptyStruct" href="#TestEmptyStruct">type TestEmptyStruct struct{}</a>

```
searchKey: testdata.TestEmptyStruct
```

```Go
type TestEmptyStruct struct{}
```

### <a id="StringAlias" href="#StringAlias">type StringAlias string</a>

```
searchKey: testdata.StringAlias
```

```Go
type StringAlias string
```

### <a id="StructTagRegression" href="#StructTagRegression">type StructTagRegression struct</a>

```
searchKey: testdata.StructTagRegression
```

```Go
type StructTagRegression struct {
	Value int `key:",range=[:}"`
}
```

StructTagRegression is a struct that caused panic in the wild. Added here to support a regression test. 

See [https://github.com/tal-tech/go-zero/blob/11dd3d75ecceaa3f5772024fb3f26dec1ada8e9c/core/mapping/unmarshaler_test.go#L2272](https://github.com/tal-tech/go-zero/blob/11dd3d75ecceaa3f5772024fb3f26dec1ada8e9c/core/mapping/unmarshaler_test.go#L2272). 

### <a id="TestEqualsStruct" href="#TestEqualsStruct">type TestEqualsStruct struct</a>

```
searchKey: testdata.TestEqualsStruct
```

```Go
type TestEqualsStruct = struct {
	Value int
}
```

### <a id="ShellStruct" href="#ShellStruct">type ShellStruct struct</a>

```
searchKey: testdata.ShellStruct
```

```Go
type ShellStruct struct {
	// Ensure this field comes before the definition
	// so that we grab the correct one in our unit
	// tests.
	InnerStruct
}
```

### <a id="InnerStruct" href="#InnerStruct">type InnerStruct struct{}</a>

```
searchKey: testdata.InnerStruct
```

```Go
type InnerStruct struct{}
```

### <a id="ParallelizableFunc" href="#ParallelizableFunc">type ParallelizableFunc func(ctx context.Context) error</a>

```
searchKey: testdata.ParallelizableFunc
```

```Go
type ParallelizableFunc func(ctx context.Context) error
```

ParallelizableFunc is a function that can be called concurrently with other instances of this function type. 

### <a id="SecretBurger" href="#SecretBurger">type SecretBurger secret.Burger</a>

```
searchKey: testdata.SecretBurger
```

```Go
type SecretBurger = secret.Burger
```

Type aliased doc 

### <a id="BadBurger" href="#BadBurger">type BadBurger struct</a>

```
searchKey: testdata.BadBurger
```

```Go
type BadBurger = struct {
	Field string
}
```

## <a id="func" href="#func">Functions</a>

```
tags: [private]
```

### <a id="useOfCompositeStructs" href="#useOfCompositeStructs">func useOfCompositeStructs()</a>

```
searchKey: testdata.useOfCompositeStructs
tags: [private]
```

```Go
func useOfCompositeStructs()
```

### <a id="Parallel" href="#Parallel">func Parallel(ctx context.Context, fns ...ParallelizableFunc) error</a>

```
searchKey: testdata.Parallel
```

```Go
func Parallel(ctx context.Context, fns ...ParallelizableFunc) error
```

Parallel invokes each of the given parallelizable functions in their own goroutines and returns the first error to occur. This method will block until all goroutines have returned. 

### <a id="Switch" href="#Switch">func Switch(interfaceValue interface{}) bool</a>

```
searchKey: testdata.Switch
```

```Go
func Switch(interfaceValue interface{}) bool
```

