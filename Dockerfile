FROM sourcegraph/src-cli:3.30.4@sha256:76ee253f9ba6ed1a8fdc46ab1e3f333ea0813841d34feb1aa9b8b57edce4eaab AS src-cli

FROM golang:1.16.7-alpine@sha256:3e6a2def9a57f23344a75bd71d9cd79726f0fbaf4b75330be5669773df0e9d4c

RUN apk add --no-cache git=2.32.0-r0

COPY --from=src-cli /usr/bin/src /usr/bin/
COPY lsif-go /usr/bin/
