FROM golang:1.11.4 as builder
WORKDIR /go/src/github.com/stuartnelson3/twemproxy_exporter
COPY . .
RUN go get && make


FROM scratch
COPY --from=builder /go/src/github.com/stuartnelson3/twemproxy_exporter/twemproxy_exporter /go/bin/twemproxy_exporter
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
ENTRYPOINT ["/go/bin/twemproxy_exporter"]
