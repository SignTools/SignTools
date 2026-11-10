# Web UI files are platform-independent, so build them once on the build platform.
FROM --platform=$BUILDPLATFORM oven/bun:1.4.4-alpine AS web

WORKDIR /src
COPY package.json bun.lock bunfig.toml ./
RUN bun install --frozen-lockfile --ignore-scripts
COPY scripts scripts
COPY src/web src/web
RUN bun run build

FROM golang:1.26.5-alpine AS builder

WORKDIR /src
COPY . .
COPY --from=web /src/src/assets/js src/assets/js
COPY --from=web /src/src/assets/vendor src/assets/vendor

RUN apk add --no-cache git && \
    go mod download && \
    CGO_ENABLED=0 go build -ldflags="-s -w" -buildvcs=false -o "SignTools"

FROM scratch

WORKDIR /

# for HTTPS calls to builders and app URLs
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
# Go stores large multipart form uploads under /tmp
COPY --from=builder --chmod=1777 /tmp /tmp
COPY --from=builder "/src/SignTools" "/"

ENTRYPOINT ["/SignTools"]
EXPOSE 8080
