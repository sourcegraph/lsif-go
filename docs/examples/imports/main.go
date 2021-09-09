package main

import (
	"fmt"
	. "net/http"
	s "sort"
)

func Main() {
	sortedStrings := []string{"hello", "world", "!"}

	// s -> sort
	s.Strings(sortedStrings)

	// http.CanonicalHeaderKey -> CanonicalHeaderKey
	fmt.Println(CanonicalHeaderKey(sortedStrings[0]))
}
