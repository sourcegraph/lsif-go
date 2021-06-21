module github.com/sourcegraph/lsif-go

go 1.15

require (
	github.com/alecthomas/kingpin v2.2.6+incompatible
	github.com/efritz/pentimento v0.0.0-20190429011147-ade47d831101
	github.com/google/go-cmp v0.5.5
	github.com/hashicorp/go-multierror v1.1.1
	github.com/hexops/autogold v1.3.0
	github.com/pkg/errors v0.9.1
	github.com/slimsag/godocmd v0.0.0-20161025000126-a1005ad29fe3
	github.com/sourcegraph/lsif-static-doc/staticdoc v0.0.0-20210618193543-40beba2f9728
	github.com/sourcegraph/sourcegraph/lib v0.0.0-20210618195625-c5188e73e214
	golang.org/x/tools v0.1.3
	mvdan.cc/gofumpt v0.1.1 // indirect
)

replace github.com/sourcegraph/lsif-static-doc => ../lsif-static-doc
