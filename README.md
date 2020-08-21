# Go LSIF indexer ![](https://img.shields.io/badge/status-ready-brightgreen)

Visit https://lsif.dev/ to learn about LSIF.

## Prerequisites

- [Go](https://golang.org/)

## Installation

```
go get github.com/sourcegraph/lsif-go/cmd/lsif-go
```

## Indexing your repository

After installing `lsif-go` onto your PATH, run the command in the root where your `go.mod` file is located.

```
$ lsif-go
✔ Loading packages... Done (1.004256336s).
✔ Emitting documents... Done (112.332µs).
✔ Adding import definitions... Done (106.871µs).
✔ Preloading hover text and moniker paths... Done (206.538662ms).
✔ Indexing definitions... Done (14.696201ms).
✔ Indexing references... Done (12.748611ms).
✔ Linking reference results to ranges... Done (7.027725ms).
✔ Emitting contains relations... Done (330.363µs).

14 package(s), 52 file(s), 1642 def(s), 33725 element(s)
Processed in 1.246392158s
```

Use `lsif-go --help` for more information.

## Updating your index

To keep your index up-to-date, you can add a step to your CI to generate new data when your repository changes. See [our documentation](https://docs.sourcegraph.com/user/code_intelligence/adding_lsif_to_workflows) on adding LSIF to your workflows.
