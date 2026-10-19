# Moteur de la version web (cmd/lattis-engine) pour lattisd : un binaire
# statique avec le frontend web embarqué, dans /usr/local/bin/lattis-engine.
#
#   docker build -f build/engine/Dockerfile -t ghcr.io/be-lattis/lattis-engine:dev .
#
# lattisd le lance une fois par session (mode serveur : --server, --token-url,
# --base-path, --dir, --workbook ; voir cmd/lattis-engine).

FROM node:22-bookworm-slim AS web
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
# Les liaisons (frontend/bindings) sont versionnées : pas de Wails ici.
RUN npm run build:web

FROM golang:1.26-bookworm AS go
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/frontend/dist-web ./cmd/lattis-engine/dist-web
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -tags web,embedweb -trimpath -ldflags "-s -w -X github.com/lalamefine/lattis/internal/bridge.Version=${VERSION}" -o /out/lattis-engine ./cmd/lattis-engine

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=go /out/lattis-engine /usr/local/bin/lattis-engine
USER nonroot
ENTRYPOINT ["/usr/local/bin/lattis-engine"]
