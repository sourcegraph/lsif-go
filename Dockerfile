FROM sourcegraph/src-cli:3.37.0@sha256:985bf866cfbd9cac6140169daf3f3b925ca5a8b4d82519b1bd847adeae574172 AS src-cli

FROM golang:1.17.7-buster@sha256:efc7c904b0f676de93fbfc2be5377eace49e2b9bdc5ab9a1b59606b0180cf774

COPY --from=src-cli /usr/bin/src /usr/bin/
COPY lsif-go /usr/bin/
