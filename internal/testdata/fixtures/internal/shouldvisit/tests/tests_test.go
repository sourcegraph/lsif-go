package tests

import "testing"

func TestFoo(t *testing.T) {
	_ = foo()
}

func BenchmarkFoo(b *testing.B) {
	_ = foobar()
}
