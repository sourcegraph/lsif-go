FROM sourcegraph/alpine:3.10

COPY lsif-go /usr/bin/
ENTRYPOINT ["/usr/bin/lsif-go"]
