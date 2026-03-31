FROM golang:1.24-bookworm AS backend

WORKDIR /app/server

COPY server/go.mod server/go.sum ./
RUN go mod download

COPY server ./

RUN mkdir -p ./internal/http/static/dist && \
  rm -rf ./internal/http/static/dist/* && \
  printf '%s\n' \
    '<!doctype html>' \
    '<html lang="es">' \
    '  <head>' \
    '    <meta charset="UTF-8" />' \
    '    <meta name="viewport" content="width=device-width, initial-scale=1.0" />' \
    '    <title>Wattless API</title>' \
    '  </head>' \
    '  <body>' \
    '    <main style="font-family: sans-serif; max-width: 42rem; margin: 4rem auto; line-height: 1.6;">' \
    '      <h1>Wattless API</h1>' \
    '      <p>Este despliegue expone la API y el scanner de Wattless.</p>' \
    '      <p>El frontend se publica por separado.</p>' \
    '    </main>' \
    '  </body>' \
    '</html>' \
    > ./internal/http/static/dist/index.html

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
