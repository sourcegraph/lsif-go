# Language Server Indexing Format Implementation for Go

🚨 This implementation is still in very early stage and follows the latest LSIF specification closely.

## Language Server Index Format

The purpose of the Language Server Index Format (LSIF) is to define a standard format for language servers or other programming tools to dump their knowledge about a workspace. This dump can later be used to answer language server [LSP](https://microsoft.github.io/language-server-protocol/) requests for the same workspace without running the language server itself. Since much of the information would be invalidated by a change to the workspace, the dumped information typically excludes requests used when mutating a document. So, for example, the result of a code complete request is typically not part of such a dump.

A first draft specification can be found [here](https://github.com/Microsoft/language-server-protocol/blob/master/indexFormat/specification.md).

## Quickstart

1. Download and build this program via `go get github.com/sourcegraph/lsif-go/cmd/lsif-go`.
2. The binary `lsif-go` should be installed into your `$GOPATH/bin` directory.
3. Make sure you have added `$GOPATH/bin` to your `$PATH` envrionment variable.
4. Go to a root directory of a Go project, then execute `lsif-go`:

```
➜ lsif-go --out data.lsif
4 package(s), 10 file(s), 597 def(s), 11521 element(s)
Processed in 770.817859ms
```

By default, the indexer will read the current directory as the root of the project.

Use `lsif-go --help` for more information

## Try it out!

Go to https://sourcegraph.com/github.com/gorilla/mux@d83b6ff/-/blob/mux.go and try to hover and jump around!

## Testing Commands

- Validate: `lsif-util validate data.lsif`
- Visualize: `lsif-util visualize data.lsif --distance 2 | dot -Tpng -o image.png`
