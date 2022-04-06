ARG GOLANG_VERSION=1.18
ARG GOLANG_OPTIONS="CGO_ENABLED=0 GOOS=linux GOARCH=amd64"

FROM docker.io/golang:${GOLANG_VERSION} as build

WORKDIR /gcp-exporter

COPY go.* ./
COPY main.go .
COPY collector ./collector
COPY gcp ./gcp

ARG VERSION=""
ARG COMMIT=""

RUN env ${GOLANG_OPTIONS} \
    go build \
    -ldflags "-X main.OSVersion=${VERSION} -X main.GitCommit=${COMMIT}" \
    -a -installsuffix cgo \
    -o /go/bin/gcp-exporter \
    ./main.go

FROM gcr.io/distroless/base-debian11

LABEL org.opencontainers.image.source https://github.com/DazWilkin/gcp-exporter

COPY --from=build /go/bin/gcp-exporter /

EXPOSE 9402

ENTRYPOINT ["/gcp-exporter"]
