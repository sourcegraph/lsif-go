FROM sourcegraph/src-cli:3.16 AS src-cli

FROM golang:1.14-buster

COPY --from=src-cli /usr/bin/src /usr/bin/
COPY lsif-go /usr/bin/
