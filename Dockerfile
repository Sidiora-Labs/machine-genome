# syntax=docker/dockerfile:1.7@sha256:a57df69d0ea827fb7266491f2813635de6f17269be881f696fbfdf2d83dda33e
FROM golang:1.24-bookworm@sha256:1a6d4452c65dea36aac2e2d606b01b4a029ec90cc1ae53890540ce6173ea77ac AS build

ARG VERSION=0.2.0
ARG COMMIT=unknown
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOFLAGS=-buildvcs=false go build \
    -trimpath \
    -ldflags="-s -w -X main.buildVersion=${VERSION} -X main.buildCommit=${COMMIT}" \
    -o /out/mgs ./cmd/mgs

FROM gcr.io/distroless/static-debian12:nonroot@sha256:aef9602f8710ec12bde19d593fed1f76c708531bb7aba205110f1029786ead7b

LABEL org.opencontainers.image.title="Machine Genome" \
      org.opencontainers.image.description="Machine Genome protocol and registry reference implementation" \
      org.opencontainers.image.source="https://github.com/paxlabs-inc/machine-genome" \
      org.opencontainers.image.licenses="Apache-2.0"

COPY --from=build /out/mgs /usr/local/bin/mgs
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/mgs"]
CMD ["help"]
