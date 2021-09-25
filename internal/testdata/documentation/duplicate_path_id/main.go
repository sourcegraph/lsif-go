package gosrc

type importMeta struct{}

type sourceMeta struct{}

func fetchMeta() (string, *importMeta, *sourceMeta) {
	panic("hmm")
}

func init() {
}

// two inits in the same file is legal
func init() {
}

// three inits in the same file is legal
func init() {
}
