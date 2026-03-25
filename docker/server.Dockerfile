FROM golang:1.24-bookworm

RUN apt-get update && apt-get install -y chromium && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY server/go.mod server/go.sum ./
RUN go mod download

COPY server ./

ENV PORT=8080
ENV BROWSER_BIN=/usr/bin/chromium

CMD ["go", "run", "./cmd/api"]

