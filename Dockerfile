FROM golang:1.14.5 AS builder
WORKDIR /go/src/app
ADD . .
RUN \
  GO111MODULE=on \
  CGO_ENABLED=0 \
  GOOS=linux \
  GOARCH=amd64 \
  go build -a -installsuffix cgo -o vault-init .

FROM alpine:3.12.0
RUN addgroup vault && \
    adduser -S -G vault vault
ADD https://curl.haxx.se/ca/cacert.pem /etc/ssl/certs/ca-certificates.crt
RUN chown root:vault -R /etc/ssl/certs
RUN chmod -R 755 /etc/ssl/certs
COPY --from=builder /go/src/app/vault-init /
CMD ["/vault-init"]
