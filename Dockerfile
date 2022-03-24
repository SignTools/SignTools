FROM golang:1.17.6-alpine AS builder

WORKDIR /src
COPY . .

RUN go mod download && \
    CGO_ENABLED=0 go build -ldflags="-s -w" -o "SignTools"

FROM alpine:3.15.2

WORKDIR /

COPY --from=builder "/src/SignTools" "/"

ENTRYPOINT ["/SignTools"]
EXPOSE 8080
