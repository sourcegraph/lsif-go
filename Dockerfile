FROM sourcegraph/src-cli:3.33.5@sha256:1b2584614c1013bd4c448e552a317a1e9a0bb6444d4ab0c1e5005facd17da395 AS src-cli

FROM golang:1.17.3-buster@sha256:c1bae5fc60e8191cbda41f8f4822570568a163a2692987a22990fba1d3e4f07b

COPY --from=src-cli /usr/bin/src /usr/bin/
COPY lsif-go /usr/bin/
