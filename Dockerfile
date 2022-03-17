FROM golang:1.18.0-alpine AS builder

WORKDIR /src
COPY . .

RUN go mod download && \
    CGO_ENABLED=0 go build -ldflags="-s -w" -o "SignTools"

FROM alpine:3.15.0

WORKDIR /

COPY --from=builder "/src/SignTools" "/"

ENTRYPOINT ["/SignTools"]
EXPOSE 8080
