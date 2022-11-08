  package initial
  
  func UsesLater() {
//     ^^^^^^^^^ definition sg/initial/UsesLater().
//     documentation func UsesLater()
   DefinedLater()
// ^^^^^^^^^^^^ reference sg/initial/DefinedLater().
  }
  
  func DefinedLater() {}
//     ^^^^^^^^^^^^ definition sg/initial/DefinedLater().
//     documentation func DefinedLater()
  
