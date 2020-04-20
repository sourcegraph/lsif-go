FROM sourcegraph/src-cli:3.11

COPY lsif-go /usr/bin/
ENTRYPOINT ["/usr/bin/lsif-go"]
