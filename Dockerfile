# Optional corporate root CAs; see certs/README.md. Empty on normal networks.
FROM --platform=$BUILDPLATFORM alpine:3.21 AS certs
COPY certs/ /certs/
RUN cat /certs/*.crt >> /etc/ssl/certs/ca-certificates.crt 2>/dev/null || true

# Build stages always run on the native architecture; emulating the Go and Node
# toolchains under QEMU corrupts downloads and crashes the compiler.
FROM --platform=$BUILDPLATFORM node:22-alpine AS web
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
ENV NODE_EXTRA_CA_CERTS=/etc/ssl/certs/ca-certificates.crt
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS build
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY --from=web /src/internal/web/dist ./internal/web/dist
ARG TARGETOS TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath -ldflags="-s -w" -o /out/sftpweb ./cmd/sftpweb

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/sftpweb /sftpweb
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/sftpweb"]
