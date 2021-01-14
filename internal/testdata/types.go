package testdata

type InnerField struct{}

type Ali struct {
	f InnerField
}

func TryMe(a Ali) {
	b := Ali{f: InnerField{}}
	println(b.f)

	c := "hello"
	println(c)

	d := &Ali{}
	println(d)

	e := []Ali{{f: InnerField{}}}
	println(e)
}
