package testdata

type Inner struct {
	X int
	Y int
	Z int
}

type Outer struct {
	Inner
	W int
}
