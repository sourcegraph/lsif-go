FROM sourcegraph/src-cli:3.17.1@sha256:3eff13f7b3e2e5294aa89f9386454c9ce49ffceac20b373eadd8206af66206d8 AS src-cli

FROM golang:1.16.2-buster@sha256:5a6302e91acb152050d661c9a081a535978c629225225ed91a8b979ad24aafcd

COPY --from=src-cli /usr/bin/src /usr/bin/
COPY lsif-go /usr/bin/
