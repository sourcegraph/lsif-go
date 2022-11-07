  package initial
  
  const MY_THING = 10
//      ^^^^^^^^ definition MY_THING.
  const OTHER_THING = MY_THING
//      ^^^^^^^^^^^ definition OTHER_THING.
//                    ^^^^^^^^ reference sg/initial/MY_THING.
  
  func usesMyThing() {
//     ^^^^^^^^^^^ definition sg/initial/usesMyThing().
   _ = MY_THING
//     ^^^^^^^^ reference sg/initial/MY_THING.
  }
  
