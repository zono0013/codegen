SHELL := /bin/bash

MIGRATE_VERSION := v4.19.1

-include .env.local
export DATABASE_URL

.DEFAULT_GOAL := help
.PHONY: help build run test lint fmt tidy docker-build docker-up docker-down clean \
        install-tools proto-gen proto-clean migration-gen migration-gen-dry \
        migrate-up migrate-down migrate-status

help: ## ターゲット一覧
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[32m%-14s\033[0m %s\n", $$1, $$2}'

build: ## バイナリをビルド
	go build -o bin/api ./cmd/api

run: ## API サーバーをローカル起動
	go run ./cmd/api

test: ## テスト実行
	go test ./...

lint: ## Lint（go vet）
	go vet ./...

fmt: ## フォーマット
	gofmt -w .

tidy: ## go.sum 整理
	go mod tidy

install-tools: ## mss-migration-gen + migrate をインストール（proto からの生成は codegen の make proto-gen。buf は不要）
	go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@$(MIGRATE_VERSION)
	go install ./cmd/mss-migration-gen

proto-gen: ## proto から生成（codegen の make proto-gen。server / front / OpenAPI をまとめて生成する）
	$(MAKE) -C ../.. proto-gen

proto-clean: ## 生成物を削除（codegen の make proto-clean。server / front / OpenAPI をまとめて消す）
	$(MAKE) -C ../.. proto-clean

migration-gen: ## entity から Postgres up migration SQL を生成 (snapshot 差分ベース)
	go run ./cmd/mss-migration-gen

migration-gen-dry: ## migration-gen の dry-run (SQL を標準出力に出すだけ)
	go run ./cmd/mss-migration-gen -dry-run

migrate-up: ## migrations を全適用 (.env.local の DATABASE_URL を使用)
	migrate -path migrations -database "$$DATABASE_URL" up

migrate-down: ## 1 ステップ戻す (現在 version の down.sql が必須)
	@out=$$(migrate -path migrations -database "$$DATABASE_URL" version 2>&1) || true; \
	if ! echo "$$out" | grep -qE '^[0-9]+$$'; then \
		echo "ERROR: migrate version 取得に失敗 (未適用 / dirty / 接続エラー など):"; \
		echo "$$out"; exit 1; \
	fi; \
	if ! ls migrations/$${out}_*.down.sql >/dev/null 2>&1; then \
		echo "ERROR: migrations/$${out}_*.down.sql が見つかりません。"; \
		echo "       down するには対応する down.sql を手書きしてから再実行してください。"; \
		exit 1; \
	fi; \
	migrate -path migrations -database "$$DATABASE_URL" down 1

migrate-status: ## 現在の migration version を表示
	migrate -path migrations -database "$$DATABASE_URL" version

docker-build: ## Docker イメージビルド
	docker build -t blog-server:latest .

docker-up: ## docker-compose up
	docker compose up --build

docker-down: ## docker-compose down
	docker compose down -v

clean: ## ビルド成果物削除
	rm -rf bin/
