  package initial
  
  import "fmt"
//        ^^^ reference github.com/golang/go fmt/
  
  type Inner struct {
//     ^^^^^ definition sg/initial/Inner#
//     documentation type Inner struct
//     documentation struct {
   X int
// ^ definition sg/initial/Inner#X.
// documentation struct field X int
//   ^^^ reference builtin/builtin builtin/int#
   Y int
// ^ definition sg/initial/Inner#Y.
// documentation struct field Y int
//   ^^^ reference builtin/builtin builtin/int#
   Z int
// ^ definition sg/initial/Inner#Z.
// documentation struct field Z int
//   ^^^ reference builtin/builtin builtin/int#
  }
  
  type Outer struct {
//     ^^^^^ definition sg/initial/Outer#
//     documentation type Outer struct
//     documentation struct {
   Inner
// ^^^^^ definition sg/initial/Outer#Inner.
// documentation struct field Inner sg/initial.Inner
// ^^^^^ reference sg/initial/Inner#
   W int
// ^ definition sg/initial/Outer#W.
// documentation struct field W int
//   ^^^ reference builtin/builtin builtin/int#
  }
  
  func useOfCompositeStructs() {
//     ^^^^^^^^^^^^^^^^^^^^^ definition sg/initial/useOfCompositeStructs().
//     documentation func useOfCompositeStructs()
   o := Outer{
// ^ definition local 0
//      ^^^^^ reference sg/initial/Outer#
    Inner: Inner{
//  ^^^^^ reference sg/initial/Outer#Inner.
//         ^^^^^ reference sg/initial/Inner#
     X: 1,
//   ^ reference sg/initial/Inner#X.
     Y: 2,
//   ^ reference sg/initial/Inner#Y.
     Z: 3,
//   ^ reference sg/initial/Inner#Z.
    },
    W: 4,
//  ^ reference sg/initial/Outer#W.
   }
  
   fmt.Printf("> %d\n", o.X)
// ^^^ reference github.com/golang/go/std/fmt/fmt/
//     ^^^^^^ reference github.com/golang/go github.com/golang/go/std/fmt/Printf().
//                      ^ reference local 0
//                        ^ reference sg/initial/Inner#X.
   fmt.Println(o.Inner.Y)
// ^^^ reference github.com/golang/go/std/fmt/fmt/
//     ^^^^^^^ reference github.com/golang/go github.com/golang/go/std/fmt/Println().
//             ^ reference local 0
//               ^^^^^ reference sg/initial/Outer#Inner.
//                     ^ reference sg/initial/Inner#Y.
  }
  
