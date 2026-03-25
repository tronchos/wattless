SHELL := /bin/bash

.PHONY: install client-install server-install dev test client-dev server-dev

install: server-install client-install

server-install:
	cd server && go mod tidy

client-install:
	cd client && npm install

server-dev:
	cd server && go run ./cmd/api

client-dev:
	cd client && npm run dev

dev:
	docker compose -f docker/compose.yml up --build

test:
	cd server && go test ./...
	cd client && npm run lint

