FROM sourcegraph/src-cli:3.33.0@sha256:0a156eee108a1605a53de046dc6caaa2a711bba786ac27735dc111e8eb1af289 AS src-cli

FROM golang:1.17.1-alpine@sha256:13919fb9091f6667cb375d5fdf016ecd6d3a5d5995603000d422b04583de4ef9

RUN apk add --no-cache git=2.32.0-r0 build-base=0.5-r2

COPY --from=src-cli /usr/bin/src /usr/bin/
COPY lsif-go /usr/bin/
