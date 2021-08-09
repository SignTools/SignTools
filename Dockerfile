FROM golang:1.16.6-alpine AS builder

WORKDIR /src
COPY . .

RUN go mod download && \
    CGO_ENABLED=0 go build -ldflags="-s -w" -o "bin-release"

FROM alpine:3.14.1

WORKDIR /

COPY --from=builder "/src/bin-release" "/"

ENTRYPOINT ["/bin-release"]
EXPOSE 8080
