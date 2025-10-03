ARG GOLANG_VERSION=1.25.1

ARG TARGETOS
ARG TARGETARCH

ARG COMMIT
ARG VERSION

FROM --platform=${TARGETARCH} docker.io/golang:${GOLANG_VERSION} AS build

WORKDIR /gcp-exporter

COPY go.* ./
COPY main.go .
COPY collector ./collector
COPY gcp ./gcp

ARG TARGETOS
ARG TARGETARCH

ARG VERSION
ARG COMMIT

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build \
    -ldflags "-X main.OSVersion=${VERSION} -X main.GitCommit=${COMMIT}" \
    -a -installsuffix cgo \
    -o /go/bin/gcp-exporter \
    ./main.go

FROM --platform=${TARGETARCH} gcr.io/distroless/static-debian12:latest

LABEL org.opencontainers.image.description="Prometheus Exporter for GCP"
LABEL org.opencontainers.image.source="https://github.com/DazWilkin/gcp-exporter"

COPY --from=build /go/bin/gcp-exporter /

EXPOSE 9402

ENTRYPOINT ["/gcp-exporter"]
