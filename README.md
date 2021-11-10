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

If lsif-go is using too much memory, try setting `--dep-batch-size=100` to only load 100 dependencies into memory at once (~1GB overhead). Lowering the batch size will decrease the overhead further, but increase the runtime a lot more because loading a batch has a fixed cost of ~500ms and each additional package loaded within a batch only adds ~10ms.

Use `lsif-go --help` for more information.

## Updating your index

To keep your index up-to-date, you can add a step to your CI to generate new data when your repository changes. See [our documentation](https://docs.sourcegraph.com/code_intelligence/how-to/adding_lsif_to_workflows) on adding LSIF to your workflows.
