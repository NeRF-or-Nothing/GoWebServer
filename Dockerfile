# Build stage
FROM golang:1.23.0-alpine3.20 AS builder
WORKDIR /app
COPY . .
RUN go build -o go-web-server ./cmd/main

# Run stage
FROM alpine:3.20
WORKDIR /app
COPY --from=builder /app/go-web-server .
COPY .env .env

EXPOSE 5000

CMD ["./go-web-server"]