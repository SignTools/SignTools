# Web UI files are platform-independent, so build them once on the build platform.
FROM --platform=$BUILDPLATFORM oven/bun:1.4.4-alpine@sha256:4cf82f5686b196504fe119db3fcd9185b191f82ece82c40680e597228ec6070a AS web

WORKDIR /src
COPY package.json bun.lock bunfig.toml ./
RUN bun install --frozen-lockfile --ignore-scripts
COPY scripts scripts
COPY src/web src/web
RUN bun run build

FROM golang:1.27.4-alpine@sha256:0a5f7fe796599ff487605987432e7619ae45beeb2fba4c2ce0b907879932aebf AS builder

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
