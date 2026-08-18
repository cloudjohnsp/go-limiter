FROM golang:1.23-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .
RUN go build -o main cmd/api/main.go

FROM alpine:latest

COPY --from=builder /app/main /usr/local/bin/