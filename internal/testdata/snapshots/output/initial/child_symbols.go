  package initial
  
  // Const is a constant equal to 5. It's the best constant I've ever written. 😹
  const Const = 5
//      ^^^^^ definition Const.
//      documentation const Const untyped int = 5
//      documentation Const is a constant equal to 5. It's the best constant I've ever written. 😹
  
  // Docs for the const block itself.
  const (
   // ConstBlock1 is a constant in a block.
   ConstBlock1 = 1
// ^^^^^^^^^^^ definition ConstBlock1.
// documentation const ConstBlock1 untyped int = 1
// documentation Docs for the const block itself.
  
   // ConstBlock2 is a constant in a block.
   ConstBlock2 = 2
// ^^^^^^^^^^^ definition ConstBlock2.
// documentation const ConstBlock2 untyped int = 2
// documentation Docs for the const block itself.
  )
  
  // Var is a variable interface.
  var Var Interface = &Struct{Field: "bar!"}
//    ^^^ definition Var.
//    documentation var Var Interface
//    documentation Var is a variable interface.
//        ^^^^^^^^^ reference sg/initial/Interface#
//                     ^^^^^^ reference sg/initial/Struct#
//                            ^^^^^ reference sg/initial/Struct#Field.
  
  // unexportedVar is an unexported variable interface.
  var unexportedVar Interface = &Struct{Field: "bar!"}
//    ^^^^^^^^^^^^^ definition unexportedVar.
//    documentation var unexportedVar Interface
//    documentation unexportedVar is an unexported variable interface.
//                  ^^^^^^^^^ reference sg/initial/Interface#
//                               ^^^^^^ reference sg/initial/Struct#
//                                      ^^^^^ reference sg/initial/Struct#Field.
  
  // x has a builtin error type
  var x error
//    ^ definition x.
//    documentation var x error
//    documentation x has a builtin error type
//      ^^^^^ reference builtin/builtin builtin/error#
  
  var BigVar Interface = &Struct{
//    ^^^^^^ definition BigVar.
//    documentation var BigVar Interface
//           ^^^^^^^^^ reference sg/initial/Interface#
//                        ^^^^^^ reference sg/initial/Struct#
   Field: "bar!",
// ^^^^^ reference sg/initial/Struct#Field.
   Anonymous: struct {
// ^^^^^^^^^ reference sg/initial/Struct#Anonymous.
    FieldA int
//  ^^^^^^ definition local 0
//         ^^^ reference builtin/builtin builtin/int#
    FieldB int
//  ^^^^^^ definition local 1
//         ^^^ reference builtin/builtin builtin/int#
    FieldC int
//  ^^^^^^ definition local 2
//         ^^^ reference builtin/builtin builtin/int#
   }{FieldA: 1337},
//   ^^^^^^ reference local 0
  }
  
  // What are docs, really?
  // I can't say for sure, I don't write any.
  // But look, a CAT!
  //
  //        |\      _,,,---,,_
  //  ZZZzz /,`.-'`'    -.  ;-;;,_
  //       |,4-  ) )-,_. ,\ (  `'-'
  //      '---''(_/--'  `-'\_)
  //
  // It's sleeping! Some people write that as `sleeping` but Markdown
  // isn't allowed in Go docstrings, right? right?!
  var (
   // This has some docs
   VarBlock1 = "if you're reading this"
// ^^^^^^^^^ definition VarBlock1.
// documentation var VarBlock1 string
// documentation What are docs, really?
  
   VarBlock2 = "hi"
// ^^^^^^^^^ definition VarBlock2.
// documentation var VarBlock2 string
// documentation What are docs, really?
  )
  
  // Embedded is a struct, to be embedded in another struct.
  type Embedded struct {
//     ^^^^^^^^ definition sg/initial/Embedded#
//     documentation type Embedded struct
//     documentation Embedded is a struct, to be embedded in another struct.
//     documentation struct {
   // EmbeddedField has some docs!
   EmbeddedField string
// ^^^^^^^^^^^^^ definition sg/initial/Embedded#EmbeddedField.
// documentation struct field EmbeddedField string
// documentation EmbeddedField has some docs!
//               ^^^^^^ reference builtin/builtin builtin/string#
   Field         string // conflicts with parent "Field"
// ^^^^^ definition sg/initial/Embedded#Field.
// documentation struct field Field string
// documentation conflicts with parent "Field"
//               ^^^^^^ reference builtin/builtin builtin/string#
  }
  
  type Struct struct {
//     ^^^^^^ definition sg/initial/Struct#
//     documentation type Struct struct
//     documentation struct {
   *Embedded
//  ^^^^^^^^ definition local 3
//  ^^^^^^^^ reference sg/initial/Embedded#
   Field     string
// ^^^^^ definition sg/initial/Struct#Field.
// documentation struct field Field string
//           ^^^^^^ reference builtin/builtin builtin/string#
   Anonymous struct {
// ^^^^^^^^^ definition sg/initial/Struct#Anonymous.
// documentation struct field Anonymous struct{FieldA int; FieldB int; FieldC int}
    FieldA int
//  ^^^^^^ definition sg/initial/Struct#Anonymous.FieldA.
//  documentation struct field FieldA int
//         ^^^ reference builtin/builtin builtin/int#
    FieldB int
//  ^^^^^^ definition sg/initial/Struct#Anonymous.FieldB.
//  documentation struct field FieldB int
//         ^^^ reference builtin/builtin builtin/int#
    FieldC int
//  ^^^^^^ definition sg/initial/Struct#Anonymous.FieldC.
//  documentation struct field FieldC int
//         ^^^ reference builtin/builtin builtin/int#
   }
  }
  
  // StructMethod has some docs!
  func (s *Struct) StructMethod() {}
//      ^ definition local 4
//         ^^^^^^ reference sg/initial/Struct#
//                 ^^^^^^^^^^^^ definition sg/initial/Struct#StructMethod().
//                 documentation func (*Struct).StructMethod()
//                 documentation StructMethod has some docs!
  
  func (s *Struct) ImplementsInterface() string { return "hi!" }
//      ^ definition local 5
//         ^^^^^^ reference sg/initial/Struct#
//                 ^^^^^^^^^^^^^^^^^^^ definition sg/initial/Struct#ImplementsInterface().
//                 documentation func (*Struct).ImplementsInterface() string
//                                       ^^^^^^ reference builtin/builtin builtin/string#
  
  func (s *Struct) MachineLearning(
//      ^ definition local 6
//         ^^^^^^ reference sg/initial/Struct#
//                 ^^^^^^^^^^^^^^^ definition sg/initial/Struct#MachineLearning().
//                 documentation func (*Struct).MachineLearning(param1 float32, hyperparam2 float32, hyperparam3 float32) float32
   param1 float32, // It's ML, I can't describe what this param is.
// ^^^^^^ definition local 7
//        ^^^^^^^ reference builtin/builtin builtin/float32#
  
   // We call the below hyperparameters because, uhh, well:
   //
   //    ,-.       _,---._ __  / \
   //   /  )    .-'       `./ /   \
   //   (  (   ,'            `/    /|
   //    \  `-"             \'\   / |
   //     `.              ,  \ \ /  |
   //   /`.          ,'-`----Y   |
   //     (            ;        |   '
   //     |  ,-.    ,-'         |  /
   //     |  | (   |        hjw | /
   //     )  |  \  `.___________|/
   //     `--'   `--'
   //
   hyperparam2 float32,
// ^^^^^^^^^^^ definition local 8
//             ^^^^^^^ reference builtin/builtin builtin/float32#
   hyperparam3 float32,
// ^^^^^^^^^^^ definition local 9
//             ^^^^^^^ reference builtin/builtin builtin/float32#
  ) float32 {
//  ^^^^^^^ reference builtin/builtin builtin/float32#
   // varShouldNotHaveDocs is in a function, should not have docs emitted.
   var varShouldNotHaveDocs int32
//     ^^^^^^^^^^^^^^^^^^^^ definition local 10
//                          ^^^^^ reference builtin/builtin builtin/int32#
  
   // constShouldNotHaveDocs is in a function, should not have docs emitted.
   const constShouldNotHaveDocs = 5
//       ^^^^^^^^^^^^^^^^^^^^^^ definition local 11
  
   // typeShouldNotHaveDocs is in a function, should not have docs emitted.
   type typeShouldNotHaveDocs struct{ a string }
//      ^^^^^^^^^^^^^^^^^^^^^ definition local 12
//                                    ^ definition local 13
//                                      ^^^^^^ reference builtin/builtin builtin/string#
  
   // funcShouldNotHaveDocs is in a function, should not have docs emitted.
   funcShouldNotHaveDocs := func(a string) string { return "hello" }
// ^^^^^^^^^^^^^^^^^^^^^ definition local 14
//                               ^ definition local 15
//                                 ^^^^^^ reference builtin/builtin builtin/string#
//                                         ^^^^^^ reference builtin/builtin builtin/string#
  
   return param1 + (hyperparam2 * *hyperparam3) // lol is this all ML is? I'm gonna be rich
//        ^^^^^^ reference local 7
//                  ^^^^^^^^^^^ reference local 8
//                                 ^^^^^^^^^^^ reference local 9
  }
  
  // Interface has docs too
  type Interface interface {
//     ^^^^^^^^^ definition sg/initial/Interface#
//     documentation type Interface interface
//     documentation Interface has docs too
//     documentation interface {
   ImplementsInterface() string
// ^^^^^^^^^^^^^^^^^^^ definition sg/initial/Interface#ImplementsInterface.
// documentation func (Interface).ImplementsInterface() string
//                       ^^^^^^ reference builtin/builtin builtin/string#
  }
  
  func NewInterface() Interface { return nil }
//     ^^^^^^^^^^^^ definition sg/initial/NewInterface().
//     documentation func NewInterface() Interface
//                    ^^^^^^^^^ reference sg/initial/Interface#
  
  var SortExportedFirst = 1
//    ^^^^^^^^^^^^^^^^^ definition SortExportedFirst.
//    documentation var SortExportedFirst int
  
  var sortUnexportedSecond = 2
//    ^^^^^^^^^^^^^^^^^^^^ definition sortUnexportedSecond.
//    documentation var sortUnexportedSecond int
  
  var _sortUnderscoreLast = 3
//    ^^^^^^^^^^^^^^^^^^^ definition _sortUnderscoreLast.
//    documentation var _sortUnderscoreLast int
  
  // Yeah this is some Go magic incantation which is common.
  //
  //   ,_     _
  //   |\\_,-~/
  //   / _  _ |    ,--.
  //  (  @  @ )   / ,-'
  //   \  _T_/-._( (
  //  /         `. \
  //  |         _  \ |
  //  \ \ ,  /      |
  //   || |-_\__   /
  //  ((_/`(____,-'
  //
  var _ = Interface(&Struct{})
//    ^ definition _.
//    documentation var _ Interface
//    documentation Yeah this is some Go magic incantation which is common.
//        ^^^^^^^^^ reference sg/initial/Interface#
//                   ^^^^^^ reference sg/initial/Struct#
  
  type _ = struct{}
//     ^ definition sg/initial/_#
//     documentation type _ = struct
//     documentation struct{}
  
  // crypto/tls/common_string.go uses this pattern..
  func _() {
//     ^ definition sg/initial/_().
//     documentation func _()
//     documentation crypto/tls/common_string.go uses this pattern..
  }
  
  // Go can be fun
  type (
   // And confusing
   X struct {
// ^ definition sg/initial/X#
// documentation type X struct
// documentation Go can be fun
// documentation struct {
    bar string
//  ^^^ definition sg/initial/X#bar.
//  documentation struct field bar string
//      ^^^^^^ reference builtin/builtin builtin/string#
   }
  
   Y struct {
// ^ definition sg/initial/Y#
// documentation type Y struct
// documentation Go can be fun
// documentation struct {
    baz float
//  ^^^ definition sg/initial/Y#baz.
//  documentation struct field baz invalid type
   }
  )
  
