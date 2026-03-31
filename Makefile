SHELL := /bin/bash

EMBEDDED_UI_DIR := server/internal/http/static/dist

.PHONY: install dev prod test build clean

install:
	cd client && npm install
	cd server && go mod tidy

dev:
	docker compose -f docker/compose.yml up --build

prod:
	docker compose -f docker/compose.prod.yml up --build

test:
	cd server && go test ./...
	cd client && npm test

build:
	cd client && npm run build
	mkdir -p $(EMBEDDED_UI_DIR)
	find $(EMBEDDED_UI_DIR) -mindepth 1 ! -name '.keep' -exec rm -rf {} +
	cp -a client/dist/. $(EMBEDDED_UI_DIR)/
	touch $(EMBEDDED_UI_DIR)/.keep
	cd server && go build -o bin/wattless ./cmd/api

clean:
	rm -rf client/dist server/bin
	find $(EMBEDDED_UI_DIR) -mindepth 1 ! -name '.keep' -exec rm -rf {} +
