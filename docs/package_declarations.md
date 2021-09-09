# Package Declarations


In general, we have used `types.*` structs that match the `types.Object`
interface. However there was no struct that represented the statement:

```go
package mypkg
```

That's the because the majority of the information is held in `types.Package`
and the corresponding definition in `packages.Package.Syntax`.

Since there was no types.PkgDeclaration or similar available, we created our own.
See [types.go](/internal/indexer/types.go)

## Definition vs. Reference

We only emit one definition for a package declaration. The way we pick this is detailed
in `findBestPackageDefinitionPath(...)`. For the `package mypkg`, only the "best" is
picked as the defintion, the other are all emitted as references. This makes sure that we
always jump to the best package declaration when jumping between packages.

For example, if we have a project that contains two files:
- [lib.go](/docs/examples/smollest/lib.go)
- [sub.go](/docs/examples/smollest/sub.go)

In this case the project is literally just two
package declarations. The lsif graph will look like this (some nodes removed):

![smollest_graph](/docs/examples/smollest/dump.svg)

NOTE: the two ranges point to the same resultSet but only one of the ranges
(the range from the `lib.go` file) is chosen as the result for the definition
request.

