<!--
###################################### READ ME ###########################################
### This changelog should always be read on `master` branch. Its contents on version   ###
### branches do not necessarily reflect the changes that have gone into that branch.   ###
##########################################################################################
-->

# Changelog

All notable changes to `lsif-go` are documented in this file.

## Unreleased (v1.5.0) changes

### Changed

- :rotating_light: Changed package module version generation to make cross-index queries accurate. Cross-linking may not work with indexes created before v1.5.0. [#152](https://github.com/sourcegraph/lsif-go/pull/152)
- Improve moniker identifiers for exported identifiers in projects with no go.mod file. [#153](https://github.com/sourcegraph/lsif-go/pull/153)

### Fixed

- Fixed definition relationship with composite structs and interfaces. [#156](https://github.com/sourcegraph/lsif-go/pull/156)

## v1.4.0

### Added

- Added const values to hover text. [#144](https://github.com/sourcegraph/lsif-go/pull/144)
- Support replace directives in go.mod. [#145](https://github.com/sourcegraph/lsif-go/pull/145)
- Infer package name from git upstream when go.mod file is absent. [#149](https://github.com/sourcegraph/lsif-go/pull/149)

### Changed

- :rotating_light: Changed moniker identifier generation to support replace directives and vanity imports. Cross-index linking will work only for indexes created on or after v1.4.0. [#145](https://github.com/sourcegraph/lsif-go/pull/145)
- Deduplicated import moniker vertices. [#146](https://github.com/sourcegraph/lsif-go/pull/146)
- Update lsif-protocol dependency. [#136](https://github.com/sourcegraph/lsif-go/pull/136)
- Avoid scanning duplicate test packages. [#138](https://github.com/sourcegraph/lsif-go/pull/138)

### Fixed

- Fix bad moniker generation for cross-index fields. [#148](https://github.com/sourcegraph/lsif-go/pull/148)

## v1.3.1

### Fixed

- Fixed type assertion panic with aliases to anonymous structs. [#134](https://github.com/sourcegraph/lsif-go/pull/134)

## v1.3.0

### Changed

- Type alias hovers now name the aliased type e.g. `type Alias = pkg.Original`. [#131](https://github.com/sourcegraph/lsif-go/pull/131)

### Fixed 

- Definition of the RHS type symbol in a type alias is no longer the type alias itself but the type being aliased. [#131](https://github.com/sourcegraph/lsif-go/pull/131)

## v1.2.0

### Changed

- :rotating_light: The `go mod download` step is no longer performed implicitly prior to loading packages. [#115](https://github.com/sourcegraph/lsif-go/pull/115)
- :rotating_light: Application flags have been updated. [#115](https://github.com/sourcegraph/lsif-go/pull/115), [#118](https://github.com/sourcegraph/lsif-go/pull/118)
  - `-v` is now for verbosity, not `--version` (use `-V` instead for version)
  - `-vv` and `-vvv` increase verbosity levels
  - `--module-root` validation is fixed and can now correctly point to a directory containing a go.mod file outside of the project root
  - Renamed flags for consistent casing:

    | Previous         | Current           | 
    | ---------------- | ----------------- |
    | `out`            | `output`          |
    | `projectRoot`    | `project-root`    |
    | `moduleRoot`     | `module-root`     |
    | `repositoryRoot` | `repository-root` |
    | `noOutput`       | `quiet`           |
    | `noProgress`     | `no-animation`    |



### Fixed

- Fixed a panic that occurs when a struct field contains certain structtag content. [#116](https://github.com/sourcegraph/lsif-go/pull/116)
- Packages with no documentation no longer have the hover text `'`. [#120](https://github.com/sourcegraph/lsif-go/pull/120)
- Fixed incorrect indexing of typeswitch. The symbolic variable in the type switch header and all it occurrences in the case clauses are now properly linked, and the hover text of each occurrence contains the refined type. [#122](https://github.com/sourcegraph/lsif-go/pull/122)

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
