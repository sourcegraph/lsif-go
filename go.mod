module github.com/sourcegraph/lsif-go

go 1.12

require (
	github.com/alecthomas/kingpin v2.2.6+incompatible
	github.com/alecthomas/template v0.0.0-20190718012654-fb15b899a751 // indirect
	github.com/alecthomas/units v0.0.0-20190924025748-f65c72e2690d // indirect
	github.com/efritz/pentimento v0.0.0-20190429011147-ade47d831101
	github.com/google/go-cmp v0.5.2
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.1 // indirect
	github.com/pkg/errors v0.9.1
	github.com/slimsag/godocmd v0.0.0-20161025000126-a1005ad29fe3
	github.com/sourcegraph/lsif-protocol v1.0.0
	golang.org/x/tools v0.0.0-20210101214203-2dba1e4ea05c
)

replace github.com/sourcegraph/lsif-protocol => ../lsif-protocol
