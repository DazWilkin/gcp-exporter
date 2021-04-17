ARG GOLANG_VERSION=1.16
ARG GOLANG_OPTIONS="CGO_ENABLED=0 GOOS=linux GOARCH=amd64"

FROM golang:${GOLANG_VERSION} as build

ARG VERSION=""
ARG COMMIT=""

WORKDIR /gcp-exporter

COPY go.* ./
COPY main.go .
COPY collector ./collector

RUN env ${GOLANG_OPTIONS} \
    go build \
    -ldflags "-X main.OSVersion=${VERSION} -X main.GitCommit=${COMMIT}" \
    -a -installsuffix cgo \
    -o /go/bin/gcp-exporter \
    ./main.go

FROM scratch

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /go/bin/gcp-exporter /

EXPOSE 9402

ENTRYPOINT ["/gcp-exporter"]
