# 既存のプロジェクトに導入する

`codegen-init` は、そのままだと `example/` のサンプル一式（`server/` の手書き層、`front/` の画面、Makefile など）をコピーする。すでに server や front があるプロジェクトでは `CONFIG_ONLY=1` を付けて、サンプルはコピーせず `codegen.yaml` だけを書いて生成物を取り込む。

ここでは、生成物を既存のコードにつなぐときに気をつけることをまとめる。生成される内容そのものは [`spec.md`](spec.md) を見る。

## 手順

```bash
git submodule add https://github.com/zono0013/codegen codegen

# サンプルはコピーせず、codegen.yaml だけを書いて生成する
make -C codegen codegen-init CONFIG_ONLY=1 SERVER_OUT=server FRONT_OUT=client/src/generated CLIENT=@/utils/client TITLE="My API"
```

- `SERVER_OUT`: 既存の Go のコードがあるディレクトリ（下の「server の出力先」を見る）
- `FRONT_OUT`: 生成した TS の置き場所
- `CLIENT`: 既存の API クライアント（下の「front の API クライアント」を見る）

省略したものはサンプルと同じ値（`server`、`front/generate`、`@/shared/api/apiClient`）になる。書き出される `codegen.yaml` は次のようになる（1 行目には説明のコメントが付く）。手で書いてもよい（パスはこのファイルからの相対）。

```yaml
proto: proto
targets:
  server:
    out: server
  front:
    out: client/src/generated
    client: '@/utils/client'
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

- 生成物が使う GORM や pgx などは、go.mod に自動では足されない。`go mod tidy` で入れる
- **go.mod の `go` ディレクティブは 1.22 以上にする**。生成する Handler は `"GET /api/tags/{id}"` のような Go 1.22 の ServeMux のパターンで登録する。`go 1.21` 以下だと古い ServeMux の解釈になり、エラーにならずにすべて 404 になる

## server の出力先

生成物は `<out>/internal/...` と `<out>/pkg/util/...` に出る。import path は、`<out>` から上に向かって最初に見つかった go.mod から決まる。

Go の internal の規則では、`<out>/internal/...` を import できるのは `<out>` の下のコードだけになる。そのため `<out>` は、生成した Handler を組み立てるコード（`main` など）がある場所、またはその上のディレクトリにする。ただし go.mod より上にはできない（同じ module の中にする）。

- 良い: `server/`（go.mod と `main.go` がここにある）
- 悪い: `server/gen/`（`server/main.go` から `server/gen/internal/di` を import できない）

既存のコードに `internal/handler`、`internal/domain/entity`、`pkg/util/errs`、`pkg/util/tx` などが既にあると、生成物と同じパッケージに混ざる。同じ名前のファイルがあると生成物で上書きされ、型名がぶつかるとコンパイルできない。

## Handler を組み立てる

生成されるのは Handler までで、その先の Usecase と Service の実装は手で書く。組み立て方は `example/server/cmd/api/main.go`（`main` と `buildHandlers`）が見本になる。

- `tx.Init(db)` で `*gorm.DB` を渡す。生成した Repository は `tx.From` で DB を取り出すので、呼ばないと panic する
- `errs.NewRDBErrorWrapper(<errs.DBErrorChecker の実装>)` を Repository に、`errs.NewErrorWriter()` を `di.NewHandlers` に渡す
- `di.NewHandlers(errWriter, <各 Usecase>...)` の Usecase の引数は、service 名の昇順に並ぶ

## HTTP のルーター

`di.Handlers.Register` は `*http.ServeMux` にルートを登録する。

Gin などの別のルーターを使っているときは、ServeMux をまとめて渡す。Gin なら、自分のルートに当たらなかったリクエストを ServeMux に回す。

```go
mux := http.NewServeMux()
handlers.Register(mux) // 生成した di.Handlers

r := gin.Default()
// ... 既存のルート ...
r.NoRoute(gin.WrapH(mux)) // Gin に無いルートは生成した Handler へ（無ければ ServeMux が 404 を返す）
```

- `r.Any("/api/v2/*path", gin.WrapH(mux))` のようにワイルドカードで渡すと、同じ前置きを持つ既存のルート（`/api/v2/users` など）とぶつかって Gin が panic する。`NoRoute` ならぶつからない
- `r.Use` で足したミドルウェアは `NoRoute` にも効く。`r.Group("/api").Use(auth)` のようなグループのミドルウェアは効かないので、認証などは `r.Use` で付けるか、ServeMux を包む `http.Handler` で付ける
- Gin の `c.Set` で入れた値は、生成した Handler（`r.Context()`）からは見えない。Handler に渡したい値は `context.Context` に入れる
- Gin は `NoRoute` に入る前にステータスを 404 にしておく。生成した Handler は `WriteHeader` を必ず呼ぶので影響しないが、同じ ServeMux に手で足すハンドラも `WriteHeader` を呼ぶ

## DB

生成する Repository・DB のエラーの判定・`migration-gen` は PostgreSQL が前提になっている。MySQL などで使うときの注意を、生成物ごとにまとめる。どれも MySQL で実際に動かしてはいない。

| 生成物 | Postgres 以外で |
| --- | --- |
| Entity（GORM のタグ） | そのまま使える。repeated のフィールドは JSON にして 1 列に入れる（`serializer:json`）。`type:jsonb` が効くのは `AutoMigrate` だけなので、自分のマイグレーションで JSON 型などの列を作る |
| Repository（`*_postgres_repository.gen.go`） | GORM だけで書いているが、MySQL では振る舞いが変わる所がある。`Upsert` / `BulkUpsert` は `ON DUPLICATE KEY UPDATE` になり、主キー以外の一意キーがぶつかっても既存の行を上書きする（Postgres ではエラー）。`Update` / `Delete` は変わった行が 0 だと NotFound にするので、MySQL のドライバーでは DSN に `clientFoundRows=true` を付ける（付けないと、値が変わらない更新が 404 になる） |
| DB のエラーの判定（`errs.DBErrorChecker` の Postgres 実装。`internal/infra/postgres`） | Postgres のエラーコードを見るので、一意制約の違反などを判定できない（409 にならず 500 になる）。`errs.DBErrorChecker`（`pkg/util/errs`）を自分で実装して、`errs.NewRDBErrorWrapper` に渡す |
| `migration-gen` | Postgres の SQL を出す。他の DB では使わず、既存のマイグレーションの仕組みでテーブルを作る |

DB そのものを差し替える方法は [`architecture.md`](architecture.md) の「DB を変えたいとき」を見る。

## front の API クライアント

API クライアントは生成しない。`targets.front.client` に書いた場所から `apiClient` を import する。axios のインスタンス（`axios.create(...)`）なら、そのまま指定できる。期待する形は [`spec.md`](spec.md) の「ランタイム」を見る。

生成した API 関数は、proto の `@http` に書いたパス（`/api/tags` など、`/` から始まる）にリクエストを送る。axios の `baseURL` にはオリジンだけを書く（`/api` まで含めると二重になる）。Next.js の `rewrites` でバックエンドに転送してもよい。

生成物は `zod` を import する。front に無ければ入れる。
