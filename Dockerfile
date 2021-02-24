FROM golang:alpine AS builder

RUN apk update && apk add --no-cache git
RUN apk --no-cache add ca-certificates

WORKDIR $GOPATH/src/crawler

COPY ./go.mod ./go.mod

RUN go mod download
RUN go mod verify

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o /go/bin/crawler ./cmd/main.go

RUN chmod +x /go/bin/crawler

FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /go/bin/crawler /go/bin/crawler

ENTRYPOINT ["/go/bin/crawler"]
