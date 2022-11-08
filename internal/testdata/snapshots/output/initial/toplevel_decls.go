  package initial
  
  const MY_THING = 10
//      ^^^^^^^^ definition MY_THING.
//      documentation const MY_THING untyped int = 10
  const OTHER_THING = MY_THING
//      ^^^^^^^^^^^ definition OTHER_THING.
//      documentation const OTHER_THING untyped int = 10
//                    ^^^^^^^^ reference sg/initial/MY_THING.
  
  func usesMyThing() {
//     ^^^^^^^^^^^ definition sg/initial/usesMyThing().
//     documentation func usesMyThing()
   _ = MY_THING
//     ^^^^^^^^ reference sg/initial/MY_THING.
  }
  
