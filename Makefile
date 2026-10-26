SHELL := /bin/bash

.DEFAULT_GOAL := help
.PHONY: help proto-gen proto-check proto-clean codegen-init test

# 生成先のプロジェクトのルート。Submodule として取り込まれていれば親リポジトリのルート、
# 単体ならサンプル（example/）。ROOT=... で上書きできる。
# 環境変数の ROOT は拾わない（別の用途で設定されていても生成先が変わらないように）。
SUPERPROJECT := $(shell git rev-parse --show-superproject-working-tree 2>/dev/null)
ifneq ($(origin ROOT),command line)
ROOT := $(or $(SUPERPROJECT),example)
endif

# arg はコマンドラインで渡した変数だけを返す（codegen-init の MODULE や CLIENT など。ROOT と同じく、環境変数は拾わない）
arg = $(if $(filter command line,$(origin $(1))),$($(1)))

help: ## ターゲット一覧
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[32m%-16s\033[0m %s\n", $$1, $$2}'
	@echo "  生成先: $(ROOT)"

proto-gen: ## proto から BE・FE・OpenAPI を生成（古い生成物は削除）
	go run ./cmd/gen -root "$(ROOT)"

proto-check: ## 生成物が proto・ジェネレータと一致し、OpenAPI が実ルートと一致するか検査（ファイルには触れない）
	go run ./cmd/gen check -root "$(ROOT)"

proto-clean: ## 生成物を全部削除（1 行目に生成マーカーを持つファイルだけ）
	go run ./cmd/gen clean -root "$(ROOT)"

codegen-init: ## Submodule として取り込んだ側で初回だけ実行: サンプルをコピーし codegen.yaml を作って生成（MODULE=<server の Go module> TITLE=<API 名>。既存のプロジェクトなら CONFIG_ONLY=1 [SERVER_OUT= FRONT_OUT= CLIENT= PROTO= OPENAPI_OUT=]）
	@if [ -z "$(SUPERPROJECT)" ] && [ "$(origin ROOT)" != "command line" ]; then \
		echo "codegen-init は codegen を Submodule として取り込んだリポジトリで実行する（make -C codegen codegen-init MODULE=...）"; exit 1; fi
	go run ./cmd/gen init -root "$(ROOT)" -codegen . \
		$(if $(call arg,MODULE),-module "$(call arg,MODULE)") $(if $(call arg,TITLE),-title "$(call arg,TITLE)") \
		$(if $(filter 1,$(call arg,CONFIG_ONLY)),-config-only) \
		$(if $(call arg,PROTO),-proto "$(call arg,PROTO)") $(if $(call arg,SERVER_OUT),-server-out "$(call arg,SERVER_OUT)") \
		$(if $(call arg,FRONT_OUT),-front-out "$(call arg,FRONT_OUT)") $(if $(call arg,CLIENT),-client "$(call arg,CLIENT)") \
		$(if $(call arg,OPENAPI_OUT),-openapi-out "$(call arg,OPENAPI_OUT)")

test: ## ジェネレータのテスト（example/ の生成物が最新かも含む）と、生成したランタイムを含む example/server のビルド・テスト
	go test -count=1 ./...
	cd example/server && go vet ./... && go test -count=1 ./...
