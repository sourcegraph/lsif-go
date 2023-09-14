FROM golang:1.21.1@sha256:0dff643e5bf836005eea93ad89e084a17681173e54dbaa9ec307fd776acab36e

RUN curl -L https://sourcegraph.com/.api/src-cli/src_linux_amd64 -o /usr/bin/src && chmod +x /usr/bin/src

COPY lsif-go /usr/bin/
