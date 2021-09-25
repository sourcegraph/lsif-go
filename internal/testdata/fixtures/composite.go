package testdata

import "fmt"

type Inner struct {
	X int
	Y int
	Z int
}

type Outer struct {
	Inner
	W int
}

func useOfCompositeStructs() {
	o := Outer{
		Inner: Inner{
			X: 1,
			Y: 2,
			Z: 3,
		},
		W: 4,
	}

	fmt.Printf("> %d\n", o.X)
}
