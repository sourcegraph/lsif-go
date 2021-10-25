FROM sourcegraph/src-cli:3.29.1@sha256:bfdec9e91fdd9d9bac4eab89c9496a9e8e027ffcac0048d56893d3747f8b7da9 AS src-cli

FROM golang:1.16-buster@sha256:d93712777042a4abf73182a29649e78368b1639b6bd5df61733708916e40d517

COPY --from=src-cli /usr/bin/src /usr/bin/
COPY lsif-go /usr/bin/
