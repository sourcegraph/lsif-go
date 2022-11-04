package initial

import "fmt"

type MyStruct struct{ f, y int }

func (m MyStruct) RecvFunction(b int) int { return m.f + b }

func SomethingElse() {
	s := MyStruct{f: 0}
	fmt.Println(s)
}
