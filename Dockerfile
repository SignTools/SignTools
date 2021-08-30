FROM golang:1.17.0-alpine AS builder

WORKDIR /src
COPY . .

RUN go mod download && \
    CGO_ENABLED=0 go build -ldflags="-s -w" -o "ios-signer-service"

FROM alpine:3.14.2

WORKDIR /

COPY --from=builder "/src/ios-signer-service" "/"

ENTRYPOINT ["/ios-signer-service"]
EXPOSE 8080
