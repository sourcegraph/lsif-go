#!/bin/bash


cat > ./internal/gomod/stdlib.go << EOT
// THIS FILE IS GENERATED. SEE ./scripts/gen_stdlib_map.sh
package gomod

// IsStandardlibPackge determines whether a package is in the standard library
// or not. At this point, it checks whether the package name is one of those
// that is found from running "go list std" in the latest released go version.
func IsStandardlibPackge(pkg string) bool {
	_, ok := standardLibraryMap[pkg]
	return ok
}

var contained = struct{}{}

// This list is calculated from "go list std".
var standardLibraryMap = map[string]interface{}{
EOT
go list std | awk '{ print "\""$0"\": contained,"}' >> ./internal/gomod/stdlib.go
echo "}" >> ./internal/gomod/stdlib.go

go fmt ./internal/gomod/stdlib.go
