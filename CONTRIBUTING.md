# Contributing

## Releasing `lsif-go`

To release a new version of lsif-go:

1. Update the [Changelog](./CHANGELOG.md) with any relevant changes
2. Create a new tag and push the tag.
  - To see the most recent tag, run: `git describe --tags --abbrev=0`
  - To push a new tag, run (while substituing your new tag): `git tag v1.7.5 && git push --tags`
3. Go to [lsif-go-action](https://github.com/sourcegraph/lsif-go-action) and update to the tagged commit.
