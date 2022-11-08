  package initial
  
  import "net/http"
//        ^^^^^^^^ reference github.com/golang/go net/http/
  
  type NestedHandler struct {
//     ^^^^^^^^^^^^^ definition sg/initial/NestedHandler#
//     documentation type NestedHandler struct
//     documentation struct {
   http.Handler
// ^^^^ reference github.com/golang/go/std/net/http/http/
//      ^^^^^^^ definition local 0
//      ^^^^^^^ reference github.com/golang/go github.com/golang/go/std/net/http/Handler#
  
   // Wow, a great thing for integers
   Other int
// ^^^^^ definition sg/initial/NestedHandler#Other.
// documentation struct field Other int
// documentation Wow, a great thing for integers
//       ^^^ reference builtin/builtin builtin/int#
  }
  
  func NestedExample(n NestedHandler) {
//     ^^^^^^^^^^^^^ definition sg/initial/NestedExample().
//     documentation func NestedExample(n NestedHandler)
//                   ^ definition local 1
//                     ^^^^^^^^^^^^^ reference sg/initial/NestedHandler#
   _ = n.Handler.ServeHTTP
//     ^ reference local 1
//       ^^^^^^^ reference local 0
//               ^^^^^^^^^ reference github.com/golang/go github.com/golang/go/std/net/http/ServeHTTP().
   _ = n.ServeHTTP
//     ^ reference local 1
//       ^^^^^^^^^ reference github.com/golang/go github.com/golang/go/std/net/http/ServeHTTP().
   _ = n.Other
//     ^ reference local 1
//       ^^^^^ reference sg/initial/NestedHandler#Other.
  }
  
