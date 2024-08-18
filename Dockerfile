# BUILD STAGE
FROM golang:1.23.0-alpine3.20 AS builder
WORKDIR /app
COPY . .
RUN go build -o /go-web-server ./cmd/main

# RUN STAGE
FROM alpine:3.20
WORKDIR /app
COPY --from=builder /go-web-server .
COPY secrets ./secrets
COPY data ./data
EXPOSE 5000
CMD ["./go-web-server"]