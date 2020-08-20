FROM sourcegraph/src-cli:3.17.1@sha256:3eff13f7b3e2e5294aa89f9386454c9ce49ffceac20b373eadd8206af66206d8 AS src-cli

FROM golang:1.14-buster@sha256:71f35a85bbd89645bc9f95abe4da751958677d66094bebfa5d9a7fcaadc8fa27

COPY --from=src-cli /usr/bin/src /usr/bin/
COPY lsif-go /usr/bin/
