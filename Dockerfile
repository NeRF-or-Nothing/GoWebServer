 # syntax=docker/dockerfile:experimental

# BUILD STAGE
FROM golang:1.23.0-alpine3.20 AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy the source from the current directory to the working directory inside the container
COPY . .

# Build the Go app
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -o /go-web-server ./cmd/main
    # go build -gcflags="all=-N -l" -o /go-web-server ./cmd/main


# RUN STAGE
FROM alpine:3.20

WORKDIR /app

COPY --from=builder /go-web-server .
COPY secrets ./secrets
# COPY data ./data

EXPOSE 5000

# For debugging
ENV GOTRACEBACK=crash

CMD ["./go-web-server"]