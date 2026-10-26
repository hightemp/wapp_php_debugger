# wapp-debugger server image: docker build -t wapp-debugger . (make docker-build)
# Build stages run on the build platform and cross-compile, so multi-arch builds need no emulation.

FROM --platform=$BUILDPLATFORM node:24-alpine AS ui
WORKDIR /src/ui
COPY ui/package.json ui/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY ui/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS server
ARG TARGETOS TARGETARCH
ARG VERSION=dev COMMIT=none BUILD_DATE=unknown
WORKDIR /src/server
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
COPY --from=ui /src/server/web/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -tags embedui -trimpath \
      -ldflags "-s -w -X github.com/hightemp/wapp_php_debugger/server/internal/version.Version=$VERSION \
        -X github.com/hightemp/wapp_php_debugger/server/internal/version.Commit=$COMMIT \
        -X github.com/hightemp/wapp_php_debugger/server/internal/version.Date=$BUILD_DATE" \
      -o /out/wapp-debugger ./cmd/wapp-debugger \
 && mkdir -p /out/data

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=server /out/wapp-debugger /wapp-debugger
# Writable for any --user (the docs run it as the owner of the project files)
COPY --from=server --chown=nonroot:nonroot --chmod=1777 /out/data /data
VOLUME /data
EXPOSE 8820
# Listening on all interfaces inside the container requires WAPP_DEBUGGER_TOKEN; arguments are appended
# (docker run … --logs-dir /var/www/app/tmp/logs), and a later --addr overrides this one
ENTRYPOINT ["/wapp-debugger", "serve", "--addr", "0.0.0.0:8820", "--db", "/data/wapp-debugger.db"]
