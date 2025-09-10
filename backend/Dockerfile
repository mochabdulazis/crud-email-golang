FROM golang:1.25.1-alpine3.22 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o app main.go

FROM alpine:latest

WORKDIR /root/
COPY --from=builder /app/app .

EXPOSE 8081

CMD ["./app"]