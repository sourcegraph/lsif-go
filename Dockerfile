FROM golang:1.18.2@sha256:800d9b4fb6231053473df14d5a7116bfd33500bca5ca4c6d544de739d9a7d302

RUN curl -L https://sourcegraph.com/.api/src-cli/src_linux_amd64 -o /usr/bin/src && chmod +x /usr/bin/src

COPY lsif-go /usr/bin/
