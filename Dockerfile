FROM sourcegraph/src-cli:3.11 AS src-cli

FROM golang:1.13.1-buster

COPY --from=src-cli $(which src) /usr/bin/
COPY lsif-go /usr/bin/
ENTRYPOINT ["/bin/sh -c"]
