# syntax=docker/dockerfile:1.7
# Mocks locales de Identidad y Productos. Sólo para docker-compose.mocks.yaml; no se publica.
FROM --platform=$BUILDPLATFORM golang:1.26.11-alpine3.23@sha256:23f198931d4bd83b4f5a613bd75656257ae8bf832dcbc2db80e1050b5219ca2a AS build
WORKDIR /src
COPY apps/mocks/ ./
ARG TARGETOS
ARG TARGETARCH
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -buildvcs=false -trimpath -ldflags="-s -w" -o /out/mocks .

FROM gcr.io/distroless/static-debian13:nonroot@sha256:f7f8f729987ad0fdf6b05eeeae94b26e6a0f613bdf46feea7fc40f7bd72953e6
WORKDIR /app
COPY --from=build --chown=nonroot:nonroot /out/mocks /app/mocks
USER nonroot:nonroot
EXPOSE 8080
HEALTHCHECK --interval=5s --timeout=3s --start-period=5s --retries=10 CMD ["/app/mocks", "-healthcheck"]
ENTRYPOINT ["/app/mocks"]
