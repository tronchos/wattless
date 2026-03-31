FROM node:20-bookworm AS frontend

WORKDIR /app/client

COPY client/package.json client/package-lock.json ./
RUN npm ci

COPY client ./
RUN npm run build

FROM golang:1.24-bookworm AS backend

WORKDIR /app/server

COPY server/go.mod server/go.sum ./
RUN go mod download

COPY server ./
COPY --from=frontend /app/client/dist ./internal/http/static/dist

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/wattless ./cmd/api

FROM debian:bookworm-slim AS runner

RUN apt-get update && apt-get install -y --no-install-recommends \
  ca-certificates \
  chromium \
  curl \
  && rm -rf /var/lib/apt/lists/* \
  && useradd --system --create-home --home-dir /home/wattless --shell /usr/sbin/nologin wattless

WORKDIR /app

ENV PORT=8080
ENV BROWSER_BIN=/usr/bin/chromium

COPY --from=backend /out/wattless ./wattless
RUN chown -R wattless:wattless /app

USER wattless

EXPOSE 8080

CMD ["./wattless"]
