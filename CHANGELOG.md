<!--
###################################### READ ME ###########################################
### This changelog should always be read on `master` branch. Its contents on version   ###
### branches do not necessarily reflect the changes that have gone into that branch.   ###
##########################################################################################
-->

# Changelog

All notable changes to `lsif-go` are documented in this file.

## Unreleased changes

Nothing yet.

## v1.1.4

### Changed

- Replaced "Preloading hover text and moniker paths" step with on-demand processing of packages. This should give a small index time speed boost and is likely to lower resident memory in some environments. [#104](https://github.com/sourcegraph/lsif-go/pull/104)

## v1.1.3

### Changed

- Additional updates to lower resident memory. [#109](https://github.com/sourcegraph/lsif-go/pull/109)

## v1.1.2

### Fixed

- Downgraded go1.15 to go1.14 in Dockerfile to help diagnose customer build issues. [5d8865d](https://github.com/sourcegraph/lsif-go/commit/5d8865d6feacb4fce3313cade2c61dc29c6271e6)

## v1.1.1

### Fixed

- Replaced the digest of the golang base image. [ae1cd6e](https://github.com/sourcegraph/lsif-go/commit/ae1cd6e97cf6551e68da9f010a3d86f438552bdb)

## v1.1.0

### Added

- Added `--verbose` flag. [#101](https://github.com/sourcegraph/lsif-go/pull/101)

### Fixed

- Fix slice out of bounds error when processing references. [#103](https://github.com/sourcegraph/lsif-go/pull/103)
- Misc updates to lower resident memory. [#105](https://github.com/sourcegraph/lsif-go/pull/105), [#106](https://github.com/sourcegraph/lsif-go/pull/106)

## v1.0.0

- Initial stable release.
