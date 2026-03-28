FROM golang:1.24-bookworm AS builder

WORKDIR /src

COPY server/go.mod server/go.sum ./
RUN go mod download

COPY server ./

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/wattless-api ./cmd/api

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

COPY --from=builder /out/wattless-api ./wattless-api
RUN chown -R wattless:wattless /app

USER wattless

EXPOSE 8080

CMD ["./wattless-api"]
