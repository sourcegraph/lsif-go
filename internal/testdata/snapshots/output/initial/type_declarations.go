  package initial
  
  type LiteralType int
//     ^^^^^^^^^^^ definition sg/initial/LiteralType#
//                 ^^^ reference builtin/builtin builtin/int#
  
  type FuncType func(LiteralType, int) bool
//     ^^^^^^^^ definition sg/initial/FuncType#
//                   ^^^^^^^^^^^ reference sg/initial/LiteralType#
//                                ^^^ reference builtin/builtin builtin/int#
//                                     ^^^^ reference builtin/builtin builtin/bool#
  
  type IfaceType interface {
//     ^^^^^^^^^ definition sg/initial/IfaceType#
   Method() LiteralType
// ^^^^^^ definition sg/initial/IfaceType#Method.
//          ^^^^^^^^^^^ reference sg/initial/LiteralType#
  }
  
  type StructType struct {
//     ^^^^^^^^^^ definition sg/initial/StructType#
   m IfaceType
// ^ definition sg/initial/StructType#m.
//   ^^^^^^^^^ reference sg/initial/IfaceType#
   f LiteralType
// ^ definition sg/initial/StructType#f.
//   ^^^^^^^^^^^ reference sg/initial/LiteralType#
  
   // anonymous struct
   anon struct {
// ^^^^ definition sg/initial/StructType#anon.
// documentation anonymous struct
    sub int
//  ^^^ definition sg/initial/StructType#anon.sub.
//      ^^^ reference builtin/builtin builtin/int#
   }
  
   // interface within struct
   i interface {
// ^ definition sg/initial/StructType#i.
// documentation interface within struct
    AnonMethod() bool
//  ^^^^^^^^^^ definition sg/initial/StructType#i.AnonMethod.
//               ^^^^ reference builtin/builtin builtin/bool#
   }
  }
  
  type DeclaredBefore struct{ DeclaredAfter }
//     ^^^^^^^^^^^^^^ definition sg/initial/DeclaredBefore#
//                            ^^^^^^^^^^^^^ definition sg/initial/DeclaredBefore#DeclaredAfter.
//                            ^^^^^^^^^^^^^ reference sg/initial/DeclaredAfter#
  type DeclaredAfter struct{}
//     ^^^^^^^^^^^^^ definition sg/initial/DeclaredAfter#
  
