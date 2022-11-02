package main

import (
	"fmt"

	"github.com/sourcegraph/lsif-go/internal/index"
)

func main() {
	fmt.Println("scip-go")
	index.Parse()
}
