  package initial
  
  func UsesBuiltin() int {
//     ^^^^^^^^^^^ definition sg/initial/UsesBuiltin().
//                   ^^^ reference builtin/builtin builtin/int#
   var x int = 5
//     ^ definition local 0
//       ^^^ reference builtin/builtin builtin/int#
   return x
//        ^ reference local 0
  }
  
