.PHONY: migrate run seed test lint build front front-install test-front

GO ?= go
BACKEND ?= backend

# Схема создаётся при старте сервера; отдельная команда нужна, только чтобы
# накатить миграции без запуска приложения.
migrate:
	cd $(BACKEND) && $(GO) run ./cmd/server -migrate-only

run:
	cd $(BACKEND) && $(GO) run ./cmd/server

seed:
	cd $(BACKEND) && $(GO) run ./cmd/seed

test:
	cd $(BACKEND) && $(GO) test ./...

lint:
	cd $(BACKEND) && $(GO) vet ./... && gofmt -l .

build:
	cd $(BACKEND) && $(GO) build -o ../bin/personal-inbox ./cmd/server

front-install:
	cd frontend && npm install

front:
	cd frontend && npm run dev

test-front:
	cd frontend && npm test
