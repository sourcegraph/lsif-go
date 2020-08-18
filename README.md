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
...........
5 package(s), 8 file(s), 689 def(s), 13681 element(s)
Processed in 1.002227943s
```

Use `lsif-go --help` for more information.

## Updating your index

To keep your index up-to-date, you can add a step to your CI to generate new data when your repository changes.  For example, you can use our [LSIF Go action workflow](https://github.com/sourcegraph/lsif-go-action) to generate and upload an index in GitHub Actions.
