# Go LSIF indexer ![](https://img.shields.io/badge/status-ready-brightgreen)

Visit https://lsif.dev/ to learn about LSIF.

## Installation

### Docker

```
docker pull sourcegraph/lsif-go:v1.1.2
```

### Binary

```
go get github.com/sourcegraph/lsif-go/cmd/lsif-go
```

## Indexing your repository

After installing `lsif-go` onto your PATH, run the command in the root where your `go.mod` file is located.

```
$ lsif-go --verbose
✔ Loading packages... Done (846.22ms).
✔ Emitting documents... Done (87.69µs).
✔ Adding import definitions... Done (163.11µs).
✔ Preloading hover text and moniker paths... Done (85.85ms).
✔ Indexing definitions... Done (15.8ms).
✔ Indexing references... Done (12.03ms).
✔ Linking items to definitions... Done (6.83ms).
✔ Emitting contains relations... Done (210.09µs).

Stats:
	Wall time elapsed:   968.11ms
	Packages indexed:    14
	Files indexed:       51
	Definitions indexed: 1657
	Elements emitted:    34040
```

Use `lsif-go --help` for more information.

## Updating your index

To keep your index up-to-date, you can add a step to your CI to generate new data when your repository changes. See [our documentation](https://docs.sourcegraph.com/user/code_intelligence/adding_lsif_to_workflows) on adding LSIF to your workflows.
