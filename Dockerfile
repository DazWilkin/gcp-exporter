ARG GOLANG_VERSION=1.21

ARG GOOS=linux
ARG GOARCH=amd64

ARG COMMIT
ARG VERSION

FROM docker.io/golang:${GOLANG_VERSION} as build

WORKDIR /gcp-exporter

COPY go.* ./
COPY main.go .
COPY collector ./collector
COPY gcp ./gcp

ARG GOOS
ARG GOARCH

ARG VERSION
ARG COMMIT

RUN CGO_ENABLED=0 GOOS=${GOOS} GOARCH=${GOARCH} \
    go build \
    -ldflags "-X main.OSVersion=${VERSION} -X main.GitCommit=${COMMIT}" \
    -a -installsuffix cgo \
    -o /go/bin/gcp-exporter \
    ./main.go

FROM gcr.io/distroless/static-debian11:latest

LABEL org.opencontainers.image.description "Prometheus Exporter for GCP"
LABEL org.opencontainers.image.source https://github.com/DazWilkin/gcp-exporter

COPY --from=build /go/bin/gcp-exporter /

EXPOSE 9402

ENTRYPOINT ["/gcp-exporter"]
