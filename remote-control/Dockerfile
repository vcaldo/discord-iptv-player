FROM golang:1.26 AS builder

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o remote-control ./cmd/remote-control/main.go

FROM ubuntu:24.04

RUN apt-get update && apt-get install -y ca-certificates

WORKDIR /root/

COPY --from=builder /app/remote-control .

CMD ["./remote-control"]