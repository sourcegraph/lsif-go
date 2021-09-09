#!/bin/bash

set -e
set -x

lsif-go

lsif-visualize dump.lsif \
    --exclude=sourcegraph:documentationResult \
    --exclude=hoverResult \
    | dot -Tsvg > dump.svg
