# 生成の仕様

`proto/` を唯一のソースに、**server（Go）・front（TypeScript）・OpenAPI** の生成物を作る。自動生成の仕様はこのファイルが正本。生成物のパス（`server/` / `front/` / `docs/openapi/` など）は生成先のプロジェクトのルート（このリポジトリでは `example/`）からの相対、ジェネレータ自身のパス（`cmd/` / `internal/`）はこのリポジトリのルートからの相対で書く。

```
proto/**/*.proto          （置き場所と出力先は codegen.yaml で決める。下は既定の配置）
   │  make proto-gen（= go run ./cmd/gen -root example）
   ├─→ server/**/*.gen.go            Entity / Repository / Mock / Postgres 実装 / DTO / Usecase interface / REST Handler / DI 配線
   │                                 + ランタイム（errs / tx / Postgres のエラー判定 / migration-gen）
   ├─→ front/generate/**/*.gen.ts    Zod スキーマ + 型 / 型付き API クライアント
   └─→ docs/openapi/**/*.gen.yaml    OpenAPI 3.0（path・schema ごとに分割）
```

Go ツールチェーンだけで動く（buf / protoc は不要）。

## ランタイム

生成コードが依存するランタイムも、proto の内容に関係なく毎回生成する（生成コードとランタイムのバージョンを必ず一致させるため）。手で編集せず、直したいところはこのリポジトリに PR を出す。

| 出力先（server の出力先から） | 内容 | 元のソース |
| --- | --- | --- |
| `pkg/util/errs/*.gen.go` | `DomainError` / RFC 9457 の Problem 出力 / RDB エラーの変換 | `internal/gogen/runtime/pkg/util/errs/` |
| `pkg/util/tx/tx.gen.go` | トランザクション境界（`tx.Run` / `tx.From`） | `internal/gogen/runtime/pkg/util/tx/` |
| `internal/infra/postgres/errors.gen.go` | `errs.DBErrorChecker` の Postgres 実装 | `internal/gogen/runtime/internal/infra/postgres/` |
| `cmd/migration-gen/*.gen.go` | entity の gorm タグから up migration SQL を作るツール | `internal/gogen/runtime/cmd/migration-gen/` |

- 元のソースは `*.go.src`。Go の複合リテラル（`{{...}}`）を含むので text/template は使わず、`__IMPORT_PATH__` だけを生成先の import path に置き換える
- **エラーの種類を増やすとき**は生成物を編集せず、パッケージ変数の初期化で `errs.Register` を呼ぶ（`var ErrorTypeTeapot = errs.Register("TEAPOT", http.StatusTeapot, "I'm a teapot")`）。`init` で呼ぶと、それより先に初期化されるパッケージ変数の `errs.New` が登録前の対応（500）を使ってしまう
- **ランタイムを変えたいとき**（別の DB に移すなど）は、このリポジトリに PR を出すかフォークする。使う側で生成物を編集しても、次の `make proto-gen` で戻る
- **TypeScript の `apiClient` は生成しない**。認証やリトライなどプロジェクトの方針を含むため、プロジェクトが持ち、生成した API 関数は `codegen.yaml` の `targets.front.client` から import する。期待する形は次のとおり（axios のインスタンスやそれを包んだものがそのまま使える）。生成した関数は戻り値の `.data` を返す
  - `get<T>(url, config?: { params?: object })` / `delete<T>(url, config?: { params?: object })` が `Promise<{ data: T }>` を返す
  - `post<T>(url, data?)` / `put<T>(url, data?)` / `patch<T>(url, data?)` が `Promise<{ data: T }>` を返す
  - 値は `import { apiClient } from '<client>'` に埋め込むので、引用符・バックスラッシュ・改行は使えない

## マイグレーション（`migration-gen`）

ランタイムとして生成する `cmd/migration-gen` は、Entity の `gorm` タグから Postgres 向けの up migration の SQL を作る。生成先のプロジェクトで `make migration-gen` から使う。

| | |
| --- | --- |
| 入力 | `internal/domain/entity/registry.gen.go` の `entity.All`（全 `@entity` の zero-value ポインタ） |
| 状態 | `migrations/.snapshot.json`（前回の schema。**git commit する**） |
| 出力 | `migrations/<YYYYMMDDHHMMSS>_<create\|alter\|drop>_<table>.up.sql`（変更のあった entity ごとに 1 ファイル。golang-migrate 互換）。同じ実行で複数の entity が変わったら、テーブル名の順に 1 秒ずつずらす |
| 適用 | `make migrate-up`（golang-migrate） |
| オプション | `-dry-run`（SQL を表示するだけ。`make migration-gen-dry`）、`-name`（変更が 1 テーブルのときだけ、ファイル名の後半を指定） |
| down | 生成しない。必要なら `<同じ timestamp>_<table>.down.sql` を手で書く |

**差分の判定**

| ケース | 出力 |
| --- | --- |
| snapshot に無い entity | `CREATE TABLE` |
| カラムの追加 | `ALTER TABLE ... ADD COLUMN` |
| カラムの削除 | `ALTER TABLE ... DROP COLUMN`（WARNING 付き） |
| 型 / NOT NULL の変更 | `ALTER COLUMN`（WARNING 付き） |
| unique の追加 / 削除 | 追加は `ADD CONSTRAINT "uq_<table>_<column>" UNIQUE`（`CREATE TABLE` / `ADD COLUMN` の中でも同じ名前を明示する）。削除は、名前ではなく「その列だけに付いた unique 制約」を `pg_constraint` から探して外す `DO` ブロック。名前を明示する前の既定の名前（`<table>_<column>_key`）や手で付けた名前でも外せる。見つからなければ失敗する（snapshot と DB の食い違いに気づけるように） |
| 複合インデックスの追加 / 変更 | `CREATE INDEX` / `DROP INDEX` |
| 主キーの変更 | SQL は出さず、WARNING だけ（手で書く） |
| entity の削除 | `DROP TABLE`（WARNING 付き） |

**規約**

- **CASCADE と DEFAULT は使わない**。依存があれば DROP は失敗する。`gorm` タグの `default:` は無視する
- NOT NULL のカラムを後から足すときは 2 段階にする（NULL 許容で追加 → 値を埋める → NOT NULL にする）。危険な操作（NOT NULL の追加、DROP、型の変更、主キーの変更）には `-- WARNING:` を付けるので、確かめてから commit する
- SQL を手で直したら `.snapshot.json` も合わせる。合わせないと、次の差分で同じ変更がまた出る
- **起動時に `AutoMigrate` を呼ばない**。生成する `AutoMigrate<Entity>` は integration test 用。GORM の制約の命名（`uni_<table>_<column>`）と、生成する SQL の命名（`uq_<table>_<column>`。以前に作ったテーブルは Postgres 既定の `<table>_<column>_key`）が違うため、`AutoMigrate` と生成する SQL を同じ DB に混ぜると、制約の状態が食い違う

## 設定ファイル（`codegen.yaml`）

生成先のプロジェクトのルートに置く（このリポジトリでは `example/codegen.yaml`）。無いと `generate` / `check` / `clean` はエラーになる。

```yaml
proto: proto                     # proto/**/*.proto を置くディレクトリ
targets:
  server:
    out: server                  # Go の生成物
  front:
    out: front/generate          # TypeScript の生成物
    client: "@/shared/api/apiClient"  # 生成した API 関数が import する apiClient
  openapi:
    out: docs/openapi            # OpenAPI の生成物
    title: My API                # info.title
    version: 0.1.0               # info.version
```

- パスはすべて `codegen.yaml` があるディレクトリからの相対で、`/` 区切りで書く。正規化してから判定する（`./server/` は `server`）。絶対パス（`C:` のようなドライブ指定を含む）、そのディレクトリの外を指すパス（`..`）、**そのディレクトリそのもの**（`.` や `server/..`）はエラーにする。そのものを許すと、古い生成物の削除がプロジェクト全体を走査してしまうため
- 出力先や、出力先から `codegen.yaml` のあるディレクトリまでの途中に別の Git リポジトリ（Submodule を含む）があるとエラーにする。codegen を Submodule として取り込んだ側が、誤って `codegen/` の中を出力先にしないため。古い生成物の削除も、別のリポジトリの中には入らない。ただし、シンボリックリンクで別のリポジトリの中を指す出力先までは検出しない
- 出力先どうしが同じ、または入れ子になっているとエラーにする（片方の古い生成物の削除が、もう片方の生成物を消しうるため）
- 未知のキーはタイプミスとみなしてエラーにする
- **Go の import path は書かない**。`targets.server.out` から `codegen.yaml` のあるディレクトリまでを遡り、最初に見つけた `go.mod` の `module` から求める（例: `server/go.mod` が `module example.com/blog/server` なら、生成物は `example.com/blog/server/internal/...` を import する。`services/go.mod` が `module example.com/svc` で `out: services/api` なら `example.com/svc/api`）。go.mod と二重に持たないため。見つからなければエラーにする（`clean` も設定を読むので、go.mod が無いと動かない）
- このリポジトリのルートには置かない。Submodule として取り込まれたときに、`codegen/` の中へ生成できてしまうため（`example_test.go` が検出する）

## コマンド

codegen（このリポジトリ）のルートで実行する。Submodule として取り込んだ側では `make -C codegen <target>`。生成先のプロジェクトのルート（`ROOT`）は次のように決まる。

- Submodule として取り込まれていれば、親リポジトリのルート（`git rev-parse --show-superproject-working-tree`）
- 単体なら `example/`
- `make ROOT=<dir> ...` で上書きできる

`server` からは `make proto-gen`、`front` からは `pnpm gen` でも呼べる（どちらも codegen の `make proto-gen` を呼ぶ）。

| コマンド | 内容 |
| --- | --- |
| `make codegen-init MODULE=<module> [TITLE=<API 名>]` | **Submodule として取り込んだ側で初回だけ**実行する。サンプル（`example/`）をコピーし、`codegen.yaml` を書き出して生成する（下記） |
| `make proto-gen` | 生成する。内容が変わったファイルだけ書き、生成されなくなった古い生成物は消す |
| `make proto-check` | 生成物が最新か（古い生成物が残っていないかを含む）、OpenAPI が妥当で Handler のルートと一致するかを検査する。ファイルには触れない |
| `make proto-clean` | 生成物を全部消す |
| `make test` | ジェネレータのテスト（`go test ./...`）。`example/` の生成物が最新かも検査する |

`make` の各ターゲットは `go run ./cmd/gen <generate|check|clean|init> -root $(ROOT)` を呼んでいる。

### `codegen-init`（`gen init`）

`example/` の Git 管理下のファイルを、使う側のプロジェクトのルートへ**一度だけ**コピーする。コピーした後は使う側のものなので、codegen を更新しても上書きしない（生成物と違って更新は受け取らない）。

- 生成物（1 行目に生成マーカーを持つファイル）はコピーせず、コピーの後に生成し直す
- Go の module（`example/server/go.mod` のもの）を `MODULE` に置き換える
- codegen を指す相対パス（`example/server/CLAUDE.md` から見た `../../` など）を、使う側での codegen の場所（`../codegen/` など）に書き換える。シンボリックリンクの向き先も同じ
- 既にあるファイルと `codegen.yaml` は、`-force` を付けない限り上書きしない
- サンプルのドメイン（Article / User）も一緒にコピーされる。不要なら、`proto/` の該当ファイル、手書きの Service / Usecase 実装とそのテスト、`server/cmd/api/main.go` の配線、`server/migrations/` の該当 SQL と `.snapshot.json`、`front` の画面を消すか書き換えてから `make proto-gen` し直す（init の最後に一覧を表示する）
- codegen 自身（`-codegen` と `-root` が同じ）にはコピーしない。codegen は `git clone` か Submodule で取り込む（コピーの対象を `git ls-files` で選ぶため）
- `codegen.yaml` はコピーと生成の最後に書き出す。途中で失敗しても、そのまま再実行できる
- 端末から実行したときは、`MODULE` / `TITLE` が無ければ聞く。AI や CI はオプションで渡す

古い生成物として消すのは、1 行目に `Code generated by make proto-gen` を持つファイルだけ。別ツールの `*.gen.*`、vendor / node_modules / 隠しディレクトリには触らない。

## 構成

| パス | 責務 |
| --- | --- |
| `cmd/gen/` | エントリポイント。全 Target を生成し終えてから書き出す |
| `internal/config/` | `codegen.yaml` の読み込みと検証 |
| `internal/pipeline/` | 何を生成するか（`Targets(cfg)`。出力先は `codegen.yaml` から受け取る）、ヘッダー付与・gofmt、書き出しと古い生成物の削除 |
| `internal/check/` | 生成物が最新か、OpenAPI と Handler のルートが一致するかの検査（`gen check`） |
| `internal/openapiload/` | 生成した OpenAPI をメモリ上で読み込む |
| `internal/protoload/` | proto のコンパイル |
| `internal/annotation/` | コメント上のアノテーションの解釈 |
| `internal/rest/` | 生成 REST Handler の振る舞いの規則（後述） |
| `internal/naming/` | 識別子の変換（DTO の JSON キーと Go コードで揃える） |
| `internal/gogen/` | server 向け。テンプレートは `generator/<kind>/output/<出力先をミラーしたパス>/*.tpl` |
| `internal/tsgen/` | front 向け |
| `internal/openapigen/` | OpenAPI 向け |
| `internal/prototest/` | テスト用ヘルパー |

各ジェネレータは「proto → 出力パスと内容」を返すだけの関数で、ヘッダー行と gofmt は `pipeline` がまとめて付ける。

## proto アノテーション

コメントに書く。1 行に複数並べてよい（`// @pk @paging`）。先頭が `@` の行は説明文（OpenAPI の description など）に出ない。

| マーカー | 位置 | 効果 |
| --- | --- | --- |
| `@entity` | message | Entity / DTO / Repository interface / Mock / Postgres 実装を生成し、`entity.All` に登録する。**`@pk` を持つ message だけが entity になる** |
| `@pk` | field | 主キー。`SelectByPK` / `Delete` / `BulkDelete` |
| `@unique` | field | `SelectBy<Field>` を追加 |
| `@paging` | field | cursor pagination の基準列。`SelectByCursor(ctx, limit, after *T)`（ASC 固定）。`@pk` か `@unique` と併記必須、型は `int64` / `int32` / `string`、1 message に 1 個まで |
| `@required` | field | trim + 非空バリデーション |
| `@email` | field | email 形式バリデーション。Zod は `.email()`、OpenAPI は `format: email` |
| `@timestamp` | `int64` field | Entity では `time.Time`（名前の `Unix` を除く）。DTO / JSON では `<name>_unix`（Unix 秒） |
| `@decimal(p,s)` | `string` field | Entity / DTO で `decimal.Decimal`、DB は `numeric(p,s)`、JSON は文字列。`@pk` などとは併用不可 |
| `@http METHOD /path` | rpc | REST のルート（`GET` / `POST` / `PUT` / `PATCH` / `DELETE`）。`{name}` は path param。**無い rpc は Handler も FE 関数も OpenAPI も作らない** |

```proto
// User は記事の投稿者。
//
// @entity
message User {
  // @pk @paging
  string id = 1;
  // @unique @email
  string email = 2;
  // @timestamp
  int64 created_at_unix = 3;
}

service UserService {
  // @http GET /api/users/{id}
  rpc GetUser(GetUserRequest) returns (GetUserResponse);
}
```

## REST Handler の振る舞い（`internal/rest`）

Handler・FE の型・OpenAPI は、この規則を `internal/rest` から取る。

**Response の形**: Handler は Response message をそのまま返さず「中身」を返す。

| Response message | HTTP レスポンス | FE の型 |
| --- | --- | --- |
| フィールド 0 個 | 204 No Content（body なし） | `void` |
| entity 1 個 | その DTO | `User` |
| `repeated` entity 1 個 | DTO の配列 | `User[]` |
| それ以外 | 全フィールドのオブジェクト（Usecase の `<Method>Output`） | そのまま |

**ステータス**: 空 → 204、`POST` → 201、それ以外 → 200。エラーは `application/problem+json`（RFC 9457、生成した `pkg/util/errs`）。

**Request の読み出し元**:

| | path param | それ以外のフィールド |
| --- | --- | --- |
| `GET` / `DELETE` | `{name}` → 常に string | query string。読めるのは scalar だけ（repeated / map / message は無視） |
| `POST` / `PUT` / `PATCH` | 同上 | JSON body |

**JSON の型**: `encoding/json` 基準（protojson ではない）。int64 も数値、キーは proto のフィールド名（snake_case）。

**未対応**（今は検出されずに壊れた出力になるので、使う前に対応が必要）:

- entity の `string` / `int32` / `int64` / `bool` 以外の scalar、enum、nested message、別パッケージの message
- Output / body に entity を含める（Go だけ DTO ではなく entity の形で JSON 化される）
- FE の GET 関数は、path 以外の全フィールドを query に載せる型になっている（Handler は scalar しか読まない）

## OpenAPI の出力

ルートは各ファイルへの `$ref` を繋ぐだけ。各ファイルの `$ref` はファイル相対なので、どのファイルからでも辿れる。

```
docs/openapi/
├── openapi.gen.yaml                     info / tags と paths・components への $ref
├── paths/api_users_{id}.gen.yaml        path 1 本分（"/api/users/{id}" → api_users_{id}）
└── components/
    ├── schemas/User.gen.yaml            schema 1 つ（entity は DTO の形）
    └── responses/Problem.gen.yaml       共通エラー応答
```

`info.title` / `version` は `codegen.yaml` の `targets.openapi.title` / `version` で設定する。

## テスト方針

t-wada の TDD に従う。**振る舞いを変えるときはテストを先に書き、Red を確認してから実装する。** リファクタは Green のまま行う。

| テスト | 何を守るか |
| --- | --- |
| `example_test.go` | 本番と同じ `Targets(cfg)` と `example/codegen.yaml` で `example/proto` を生成し、`example/` の生成物が最新で古い生成物が残っていないこと、生成した OpenAPI が妥当で Handler のルートとメソッド・パス・operationId が 1:1 で一致すること（`gen check` と同じ検査） |
| `cmd/gen/main_test.go` | `generate` / `check` / `clean` サブコマンドの振る舞い |
| `internal/*/..._test.go` | 各規則の単体テスト。小さな proto をテスト内に書いて動かす |

出力を意図的に変えるときは次の順で進める:

1. ジェネレータやテンプレートを変える
2. `make test` が `example_test.go` で落ちることを確認する（Red）
3. `make proto-gen` で再生成し、`git diff` で差分が意図どおりか確かめる
4. `make test` が通る（Green）

## ジェネレータを変更・追加するとき

- アノテーションと REST の振る舞いの解釈は `annotation` / `rest` に置き、ジェネレータ側に持たない
- 出力先を増やすときは `internal/pipeline.Targets(cfg)` に足し、出力先の設定を `internal/config` に追加する。出力先の重なりは `config` の検証に加える
