FROM sourcegraph/src-cli:3.16@sha256:77bb8714e0eee04a12348696892f21a84b1ba2adee94ecc53683ca8e2d66cc86 AS src-cli

FROM golang:1.14-buster@sha256:71f35a85bbd89645bc9f95abe4da751958677d66094bebfa5d9a7fcaadc8fa27

COPY --from=src-cli /usr/bin/src /usr/bin/
COPY lsif-go /usr/bin/
