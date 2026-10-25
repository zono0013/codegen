# 既存のプロジェクトに導入する

`codegen-init` は `example/` のサンプル一式（`server/` の手書き層、`front/` の画面、Makefile など）をコピーする。すでに server や front があるプロジェクトでは、サンプルはコピーせず、`codegen.yaml` だけを書いて生成物を取り込む。

ここでは、生成物を既存のコードにつなぐときに気をつけることをまとめる。生成される内容そのものは [`spec.md`](spec.md) を見る。

## 手順

```bash
git submodule add https://github.com/zono0013/codegen codegen
```

ルートに `codegen.yaml` を書く（パスはこのファイルからの相対）。

```yaml
proto: proto
targets:
  server:
    out: server                 # 既存の Go のコードがあるディレクトリ（下の「server の出力先」を見る）
  front:
    out: client/src/generated   # 生成した TS の置き場所
    client: "@/utils/client"    # 既存の API クライアント（下の「front の API クライアント」を見る）
  openapi:
    out: docs/openapi
    title: My API
    version: 0.1.0
```

proto を書いて生成し、生成物が使うライブラリを go.mod に入れる。

```bash
make -C codegen proto-gen
(cd server && go mod tidy && go build ./...)
```

生成物が使う GORM や pgx などは、go.mod に自動では足されない。`go mod tidy` で入れる。go.mod の `go` ディレクティブが古いと、`tidy` が上げることがある。

## server の出力先

生成物は `<out>/internal/...` と `<out>/pkg/util/...` に出る。import path は、`<out>` から上に向かって最初に見つかった go.mod から決まる。

Go の internal の規則では、`<out>/internal/...` を import できるのは `<out>` の下のコードだけになる。そのため `<out>` は、生成した Handler を組み立てるコード（`main` など）がある場所、またはその上のディレクトリにする。

- ✅ `server/`（go.mod と `main.go` がここにある）
- ❌ `server/gen/`（`server/main.go` から `server/gen/internal/di` を import できない）

既存のコードに `internal/handler` や `internal/domain/entity` などが既にあると、生成物と同じパッケージに混ざる。生成物のファイル名は `*.gen.go` なので上書きはしない。ただし型名がぶつかるとコンパイルできない。

## HTTP のルーター

生成する Handler は `net/http` のハンドラで、`di.Handlers.Register` が `*http.ServeMux` にルートを登録する（`"GET /api/tags/{id}"` のような Go 1.22 以降のパターン）。

Gin などの別のルーターを使っているときは、ServeMux をまとめて渡す。Gin なら、自分のルートに当たらなかったリクエストを ServeMux に回す。

```go
mux := http.NewServeMux()
handlers.Register(mux) // 生成した di.Handlers

r := gin.Default()
// ... 既存のルート ...
r.NoRoute(gin.WrapH(mux)) // Gin に無いルートは生成した Handler へ（無ければ ServeMux が 404 を返す）
```

`r.Any("/api/v2/*path", gin.WrapH(mux))` のようにワイルドカードで渡すと、同じ前置きを持つ既存のルートとぶつかって Gin が panic することがある。`NoRoute` ならぶつからない。`r.Use` で足したミドルウェア（認証など）は `NoRoute` にも効く。

## DB

生成する Repository・DB のエラーの判定・`migration-gen` は PostgreSQL が前提になっている。

| 生成物 | Postgres 以外で |
| --- | --- |
| Entity（GORM のタグ） | ほぼそのまま使える。repeated のフィールドは `jsonb` を指定するので、Postgres 以外では型を変える必要がある |
| Repository（`*_postgres_repository.gen.go`） | GORM だけで書いているので、GORM が扱える DB なら動く見込み（MySQL では未検証） |
| `errs.DBErrorChecker`（`internal/infra/postgres`） | Postgres のエラーコードを見るので、MySQL などでは一意制約の違反などを判定できない（409 にならず 500 になる）。`errs.DBErrorChecker` を自分で実装して、`errs.NewRDBErrorWrapper` に渡す |
| `migration-gen` | Postgres の SQL を出す。他の DB では使わず、既存のマイグレーションの仕組みでテーブルを作る |

DB そのものを差し替える方法は [`architecture.md`](architecture.md) の「DB を変えたいとき」を見る。

## front の API クライアント

API クライアントは生成しない。`targets.front.client` に書いた場所から `apiClient` を import する。axios のインスタンス（`axios.create(...)`）なら、そのまま指定できる。期待する形は [`spec.md`](spec.md) の「ランタイム」を見る。

生成した API 関数は `/api/...` のような相対パスにリクエストを送る。axios の `baseURL`、または Next.js の `rewrites` で、バックエンドに届くようにする。

生成物は `zod` を import する。front に無ければ入れる。
