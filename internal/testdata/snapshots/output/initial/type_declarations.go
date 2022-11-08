  package initial
  
  type LiteralType int
//     ^^^^^^^^^^^ definition sg/initial/LiteralType#
//     documentation int
//                 ^^^ reference builtin/builtin builtin/int#
  
  type FuncType func(LiteralType, int) bool
//     ^^^^^^^^ definition sg/initial/FuncType#
//     documentation func(LiteralType, int) bool
//                   ^^^^^^^^^^^ reference sg/initial/LiteralType#
//                                ^^^ reference builtin/builtin builtin/int#
//                                     ^^^^ reference builtin/builtin builtin/bool#
  
  type IfaceType interface {
//     ^^^^^^^^^ definition sg/initial/IfaceType#
//     documentation type IfaceType interface
//     documentation interface {
   Method() LiteralType
// ^^^^^^ definition sg/initial/IfaceType#Method.
// documentation func (IfaceType).Method() LiteralType
//          ^^^^^^^^^^^ reference sg/initial/LiteralType#
  }
  
  type StructType struct {
//     ^^^^^^^^^^ definition sg/initial/StructType#
//     documentation type StructType struct
//     documentation struct {
   m IfaceType
// ^ definition sg/initial/StructType#m.
// documentation struct field m sg/initial.IfaceType
//   ^^^^^^^^^ reference sg/initial/IfaceType#
   f LiteralType
// ^ definition sg/initial/StructType#f.
// documentation struct field f sg/initial.LiteralType
//   ^^^^^^^^^^^ reference sg/initial/LiteralType#
  
   // anonymous struct
   anon struct {
// ^^^^ definition sg/initial/StructType#anon.
// documentation struct field anon struct{sub int}
// documentation anonymous struct
    sub int
//  ^^^ definition sg/initial/StructType#anon.sub.
//  documentation struct field sub int
//      ^^^ reference builtin/builtin builtin/int#
   }
  
   // interface within struct
   i interface {
// ^ definition sg/initial/StructType#i.
// documentation struct field i interface{AnonMethod() bool}
// documentation interface within struct
    AnonMethod() bool
//  ^^^^^^^^^^ definition sg/initial/StructType#i.AnonMethod.
//  documentation func (interface).AnonMethod() bool
//               ^^^^ reference builtin/builtin builtin/bool#
   }
  }
  
  type DeclaredBefore struct{ DeclaredAfter }
//     ^^^^^^^^^^^^^^ definition sg/initial/DeclaredBefore#
//     documentation type DeclaredBefore struct
//     documentation struct {
//                            ^^^^^^^^^^^^^ definition sg/initial/DeclaredBefore#DeclaredAfter.
//                            documentation struct field DeclaredAfter sg/initial.DeclaredAfter
//                            ^^^^^^^^^^^^^ reference sg/initial/DeclaredAfter#
  type DeclaredAfter struct{}
//     ^^^^^^^^^^^^^ definition sg/initial/DeclaredAfter#
//     documentation type DeclaredAfter struct
//     documentation struct{}
  
