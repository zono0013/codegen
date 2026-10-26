---
name: pr
description: codegen の変更をコミットし、Pull Request を出す。ブランチ名・コミット・PR 本文の型と、出す前の確認（Submodule E2E のローカルでの再現を含む）。「PR 出して」「PR 立てて」「コミットして PR に」などで起動。dev-manager の pr の段階。
---

# PR

規約（ブランチ名、Conventional Commits、スカッシュマージ）は `CONTRIBUTING.md` が正。ここでは出し方の手順だけを書く。

## 1. ブランチとコミット

- ブランチは `main` から切る: `feat/<Issue番号>-<topic>` / `fix/<Issue番号>-<topic>` / `docs/<Issue番号>-<topic>`
- コミットの件名は Conventional Commits の日本語。本文に「なぜ」を書き、最後に `Closes #N`（途中のコミットは `Refs #N`）
- 生成物を変えたら `make proto-gen` を実行し、生成物も同じコミットに入れる
- スカッシュマージで PR のタイトルが main のコミットになる。途中のコミットはまとめなくてよい

## 2. 出す前の確認

```bash
go test ./...                                   # ジェネレータ
(cd example/server && go vet ./... && go test ./...)
make proto-check                                # example/ の生成物が最新か
```

次のどれかを変えたら、Submodule E2E をローカルで再現する（コミットしてから）。

- ジェネレータ・テンプレート・ランタイム（生成物が使う側でコンパイルできるかに関わる）
- `.github/workflows/ci.yml` の `submodule-e2e`
- README の導入手順

```bash
.claude/skills/pr/scripts/e2e-local.py            # 今のブランチ
.claude/skills/pr/scripts/e2e-local.py <branch>
```

失敗したら、表示された作業ディレクトリで原因を見る。先の PR に依存している失敗（その修正がまだ main に無い）なら、先の PR のマージを待って rebase する。

`example/front` を変えたら `cd example/front && pnpm typecheck && pnpm lint && pnpm test --run`。

## 3. PR 本文

`.github/pull_request_template.md` の見出し（Summary / Changes / Test plan / Risk）に沿って、日本語で書く。

- **Summary**: `Closes #N`。何を・**なぜ**変えるか。意思決定の経緯（ほかの案、やめたこと、前提を変えた理由）はここに書く（README や docs には書かない）
- **Changes**: ファイルや層ごとの変更。生成物がどう変わるか（`example/` の差分）
- **Test plan**: 実行したものに `[x]`。手元で確かめられなかったもの（例: Docker が動かず Postgres のテストは CI 任せ）は `[ ]` のまま理由を書く
- **Risk**: 使う側（Submodule として取り込んでいるプロジェクト）への影響。互換性を壊すなら件名を `feat!:` にし、本文に `BREAKING CHANGE:` を書く

本文の最後に、Claude Code で作った旨の 1 行を入れる。

```bash
git push -u origin <branch>
gh pr create -R zono0013/codegen -B main -t "<コミットの件名と同じ>" -b "$(cat <<'EOF'
...
EOF
)"
```

## 4. 出した後

そのまま `review-merge` に進む。レビューの指摘で直したら、追加のコミットとして push する（force-push は rebase のときだけ。`--force-with-lease` を使う）。
