# syntax=docker/dockerfile:1.7
FROM golang:1.26-bookworm AS build

ARG VERSION=0.1.1
ARG COMMIT=unknown
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOFLAGS=-buildvcs=false go build \
    -trimpath \
    -ldflags="-s -w -X main.buildVersion=${VERSION} -X main.buildCommit=${COMMIT}" \
    -o /out/mgs ./cmd/mgs

FROM gcr.io/distroless/static-debian12:nonroot

LABEL org.opencontainers.image.title="Machine Genome" \
      org.opencontainers.image.description="Machine Genome protocol and registry reference implementation" \
      org.opencontainers.image.source="https://github.com/paxlabs-inc/machine-genome" \
      org.opencontainers.image.licenses="Apache-2.0"

COPY --from=build /out/mgs /usr/local/bin/mgs
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/mgs"]
CMD ["help"]
