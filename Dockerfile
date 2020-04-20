FROM sourcegraph/src-cli:3.11.1 AS src-cli

FROM golang:1.13.1-buster

COPY --from=src-cli /usr/bin/src /usr/bin/
COPY lsif-go /usr/bin/
