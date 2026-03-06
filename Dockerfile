# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install protoc and dependencies
RUN apk add --no-cache protobuf-dev make

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Generate proto files
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest && \
    export PATH="$PATH:$(go env GOPATH)/bin" && \
    protoc --go_out=. --go_opt=paths=source_relative \
           --go-grpc_out=. --go-grpc_opt=paths=source_relative \
           api/proto/weather.proto

RUN CGO_ENABLED=0 GOOS=linux go build -o /weather-service ./cmd/server

# Runtime stage
FROM alpine:3.19

RUN apk add --no-cache ca-certificates

COPY --from=builder /weather-service /weather-service

EXPOSE 50051

CMD ["/weather-service"]
