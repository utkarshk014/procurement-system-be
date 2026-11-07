# Build stage — runs on the build host's arch and cross-compiles for the target.
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="-w -s" -o pms-api ./src && \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="-w -s" -o pms-admin ./src/cmd/admin

# Runtime stage — distroless, non-root, no shell.
FROM gcr.io/distroless/static:nonroot

COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /build/pms-api /pms-api
COPY --from=builder /build/pms-admin /pms-admin

ENV HTTP_PORT=8080
EXPOSE 8080

ENTRYPOINT ["/pms-api"]
