FROM golang:alpine AS builder

RUN apk update && apk add --no-cache git
WORKDIR /app

COPY . .
RUN GOPROXY=direct GOOS=linux GOOARCH=amd64 go build -o dyndns

FROM alpine
COPY --from=builder /app/dyndns /usr/local/bin/dyndns

CMD ["/usr/local/bin/dyndns"]
