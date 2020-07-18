FROM sourcegraph/src-cli:3.16.1@sha256:b5dd688d25557eaa5fb0ec33cf2cc15a87bc72a7f5d9efa6d5e461644e93ac09 AS src-cli

FROM golang:1.14-buster@sha256:71f35a85bbd89645bc9f95abe4da751958677d66094bebfa5d9a7fcaadc8fa27

COPY --from=src-cli /usr/bin/src /usr/bin/
COPY lsif-go /usr/bin/
