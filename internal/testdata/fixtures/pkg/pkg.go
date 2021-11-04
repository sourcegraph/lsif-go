package pkg

type Foo struct{}

func (f Foo) nonExportedMethod() {
}

func (f Foo) ExportedMethod() {
}
