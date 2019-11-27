# Go LSIF indexer

Visit https://lsif.dev/ to learn about LSIF.

## Prerequisites

- [Go](https://golang.org/)

## Installation

```
go get github.com/sourcegraph/lsif-go/cmd/lsif-go
```

## Indexing your repository

```
$ lsif-go
...........

5 package(s), 8 file(s), 689 def(s), 13681 element(s)
Processed in 1.002227943s
```

By default, the indexer will read the current directory as the root of the project.

Use `lsif-go --help` for more information.

## Development

Testing commands:

- Validate: `lsif-util validate data.lsif`
- Visualize: `lsif-util visualize data.lsif --distance 2 | dot -Tpng -o image.png`
