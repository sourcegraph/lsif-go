FROM sourcegraph/src-cli:3.17.1@sha256:3eff13f7b3e2e5294aa89f9386454c9ce49ffceac20b373eadd8206af66206d8 AS src-cli

FROM golang:1.15-buster@sha256:1f1df144b9c004b11b3ef5c8e5348d16b27ac9b6602eacca0d92045e26485c53

COPY --from=src-cli /usr/bin/src /usr/bin/
COPY lsif-go /usr/bin/
