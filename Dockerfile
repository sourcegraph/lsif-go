FROM sourcegraph/src-cli:3.37.0@sha256:985bf866cfbd9cac6140169daf3f3b925ca5a8b4d82519b1bd847adeae574172 AS src-cli

FROM golang:1.18.2@sha256:800d9b4fb6231053473df14d5a7116bfd33500bca5ca4c6d544de739d9a7d302

COPY --from=src-cli /usr/bin/src /usr/bin/
COPY lsif-go /usr/bin/
