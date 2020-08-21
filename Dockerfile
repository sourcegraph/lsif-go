FROM sourcegraph/src-cli:3.17.1@sha256:3eff13f7b3e2e5294aa89f9386454c9ce49ffceac20b373eadd8206af66206d8 AS src-cli

FROM golang:1.15-buster@sha256:3ec6a0096380c5762f4d75562788e460d4549e1b6c859449b3906358e2e4ebbf

COPY --from=src-cli /usr/bin/src /usr/bin/
COPY lsif-go /usr/bin/
