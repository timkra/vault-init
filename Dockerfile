FROM golang:1.13.5 AS builder
WORKDIR /go/src/app
ADD . .
RUN \
  GO111MODULE=on \
  CGO_ENABLED=0 \
  GOOS=linux \
  GOARCH=amd64 \
  go build -a -installsuffix cgo -mod vendor -o vault-init .

FROM scratch
ADD https://curl.haxx.se/ca/cacert.pem /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /go/src/app/vault-init /
CMD ["/vault-init"]
