  package initial
  
  import "net/http"
//        ^^^^^^^^ reference github.com/golang/go net/http/
  
  type NestedHandler struct {
//     ^^^^^^^^^^^^^ definition sg/initial/NestedHandler#
   http.Handler
// ^^^^ reference github.com/golang/go/std/net/http/http/
//      ^^^^^^^ definition sg/initial/NestedHandler#Handler.
//      ^^^^^^^ reference github.com/golang/go github.com/golang/go/std/net/http/Handler#
   Other int
// ^^^^^ definition sg/initial/NestedHandler#Other.
//       ^^^ reference builtin/builtin builtin/int#
  }
  
  func NestedExample(n NestedHandler) {
//     ^^^^^^^^^^^^^ definition sg/initial/NestedExample().
//                   ^ definition local 0
//                     ^^^^^^^^^^^^^ reference sg/initial/NestedHandler#
   _ = n.Handler.ServeHTTP
//     ^ reference local 0
//       ^^^^^^^ reference sg/initial/NestedHandler#Handler.
//               ^^^^^^^^^ reference github.com/golang/go github.com/golang/go/std/net/http/ServeHTTP().
   _ = n.ServeHTTP
//     ^ reference local 0
//       ^^^^^^^^^ reference github.com/golang/go github.com/golang/go/std/net/http/ServeHTTP().
   _ = n.Other
//     ^ reference local 0
//       ^^^^^ reference sg/initial/NestedHandler#Other.
  }
  
