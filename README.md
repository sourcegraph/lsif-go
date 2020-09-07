# Go LSIF indexer ![](https://img.shields.io/badge/status-ready-brightgreen)

Visit https://lsif.dev/ to learn about LSIF.

## Installation

Binary downloads are available on the [releases tab](https://github.com/sourcegraph/lsif-go/releases).

### Installation: Linux

```
curl -L  https://github.com/sourcegraph/lsif-go/releases/download/v1.2.0/src_linux_amd64 -o /usr/local/bin/lsif-go
chmod +x /usr/local/bin/lsif-go
```

### Installation: MacOS

```
curl -L  https://github.com/sourcegraph/lsif-go/releases/download/v1.2.0/src_darwin_amd64 -o /usr/local/bin/lsif-go
chmod +x /usr/local/bin/lsif-go
```

### Installation: Docker

```
docker pull sourcegraph/lsif-go:v1.2.0
```

## Indexing your repository

After installing `lsif-go` onto your PATH, run the command in the root where your `go.mod` file is located.

```
$ lsif-go -v
✔ Loading packages... Done (753.22ms)
✔ Emitting documents... Done (72.76µs)
✔ Adding import definitions... Done (86.24µs)
✔ Indexing definitions... Done (16.83ms)
✔ Indexing references... Done (93.36ms)
✔ Linking items to definitions... Done (8.46ms)
✔ Emitting contains relations... Done (294.13µs)

Stats:
	Wall time elapsed:   873.2ms
	Packages indexed:    14
	Files indexed:       53
	Definitions indexed: 1756
	Elements emitted:    35718
	Packages traversed:  40
```

Use `lsif-go --help` for more information.

## Updating your index

To keep your index up-to-date, you can add a step to your CI to generate new data when your repository changes. See [our documentation](https://docs.sourcegraph.com/user/code_intelligence/adding_lsif_to_workflows) on adding LSIF to your workflows.
