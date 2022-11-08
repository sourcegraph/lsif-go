  package initial
  
  func UsesLater() {
//     ^^^^^^^^^ definition sg/initial/UsesLater().
   DefinedLater()
// ^^^^^^^^^^^^ reference sg/initial/DefinedLater().
  }
  
  func DefinedLater() {}
//     ^^^^^^^^^^^^ definition sg/initial/DefinedLater().
  
