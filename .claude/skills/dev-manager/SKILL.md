---
name: dev-manager
description: codegen の開発を Issue → 実装 → PR → レビュー → マージまで進めるときの流れと制約。issue / pr / review-merge の各 Skill をこの順で使う。「開発進めて」「この Issue やって」「直して PR まで」「問題があれば Issue を立てて直して」などで起動。個々の手順だけなら各 Skill を直接使う。
---

# Dev Manager

codegen の変更を、Issue から main へのマージまで運ぶ。手順の中身は各 Skill が正で、ここは**順番と、どの段階でも守る制約**だけを持つ。

```
issue        Issue を立てる（粒度を切る）
  ↓
実装          ブランチを切り、テストを先に書いて実装する
  ↓
pr           コミットを整え、PR を出す
  ↓
review-merge サブエージェントのレビュー、CI と SonarQube、指摘への対応、スカッシュマージ、Issue の close
```

## 制約（どの段階でも守る）

### 進め方

- **Issue を先に立てる。** 見つかった問題は、直す前に Issue にする（`issue`）。1 つの Issue = 1 つの PR を基本にする
- **main には直接 push しない**（ルールセットでも塞がれている）。必ずブランチ → PR
- **振る舞いを変えるときは、テストを先に書いて落ちることを確かめてから実装する。** ジェネレータ・テンプレート・ランタイム（`internal/gogen/runtime/**/*.go.src`）・`example/proto` を変えたら `make proto-gen` で生成物も作り直してコミットする。生成物（`*.gen.*`）は手で編集しない
- **生成物がコンパイルできるかは、文字列のテストだけで済ませない。** Submodule E2E（`pr/scripts/e2e-local.py`）か、`example/` での `go vet` で確かめる
- **一つの PR に一つの関心事。** 途中で別の問題を見つけたら、その PR に混ぜず Issue を立てる。ただし、その PR の変更そのものが原因の問題は同じ PR で直す

### 書く場所

- **意思決定の経緯（なぜ変えたか、何をやめたか、何をしないか）は PR の本文と Issue に書く。** README・docs・コードコメントには、今の使い方と仕様だけを書く。コードコメントの WHY（その実装である理由）は書いてよい
- 規約は `CONTRIBUTING.md` が正。Skill に同じことを書き写さない

### 止まって確かめる

- **ユーザーが途中で方針を変えたり、前提を問い直したりしたら、作業を止める。** 進めている PR やマージを続けず、方針を整理して提案し、合意してから再開する
- 次のときは、実行する前に確かめる
  - Issue や PR を閉じる
  - 機能を消す
  - マージ済みの変更を戻す
  - 依頼の範囲を広げる
- ユーザーのほかのリポジトリ（手元の clone を含む）は書き換えない。試用は scratchpad に clone して行う

### マージの条件

`review-merge` の条件を満たしたら、ユーザーに聞かずにマージしてよい（ユーザーから許可済み）。

- サブエージェントのレビューで、マージを止める指摘が無い（指摘は対応するか、見送る理由を PR にコメントする）
- CI の必須チェックがすべて通っている
- SonarQube の Quality Gate が OK で、未解決の指摘が 0 件
- スカッシュマージ。ブランチは消す

## 段階ごとの完了条件

| 段階 | 次に進む条件 |
| --- | --- |
| issue | Issue の URL がある。大きな問題は、単独で直せる粒度に分かれている |
| 実装 | 追加したテストが、修正前に落ち、修正後に通る。`go test ./...` と `example/server` の `go vet` / `go test` が通る |
| pr | PR が Issue に `Closes #N` で紐付いている。生成物・CI を変えたなら `e2e-local.py` が通っている |
| review-merge | マージ済み。Issue が close されている |

## 並行して進めるとき

- 独立した Issue は、ブランチを分けて並行してよい。レビューの待ち時間に次の Issue を進める
- 同じファイル（特に `.github/workflows/ci.yml`、`internal/gogen/gogen_test.go`）を触る PR は、先の PR をマージしてから rebase する。rebase の衝突は、片方を捨てずに両方の変更を残す
- 先の PR の修正に依存する確認（例: E2E）は、先の PR がマージされてから行う

## いつ使わないか

- 文言だけの修正など、レビューの観点が無い小さな変更 → Issue を省き、`pr` → `review-merge` だけでよい
- 調べるだけ → Skill を使わず調べて答える
- Renovate の PR → `review-merge` だけ

## 関連

- `../issue/SKILL.md`
- `../pr/SKILL.md`
- `../review-merge/SKILL.md`
- `../../../CONTRIBUTING.md`（規約の正）
