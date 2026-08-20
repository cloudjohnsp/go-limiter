FROM golang:1.26-alpine AS builder

WORKDIR /app
    
COPY go.mod ./
RUN go mod download

COPY . .
RUN go build -o main cmd/api/main.go

FROM alpine:latest

COPY --from=builder /app/main /usr/local/bin/main

EXPOSE 8080
CMD ["/usr/local/bin/main"]