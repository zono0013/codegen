---
name: review-merge
description: codegen の PR をサブエージェントにレビューさせ、CI と SonarQube を確かめ、指摘に対応してスカッシュマージし、Issue を close する。Renovate の PR にも使う。「レビューしてマージ」「PR を片付けて」「Sonar 見て」「マージして」などで起動。dev-manager の最後の段階。
---

# Review & Merge

## 1. サブエージェントにレビューさせる

PR を出したら、すぐにバックグラウンドのサブエージェント（`general-purpose`）にレビューを頼む。待っている間に、ほかの作業を進めてよい。

依頼文に入れること:

- **読み取りだけ。** リポジトリのファイルを変えない、push しない、GitHub にコメントしない。試すなら scratchpad にコピーして行う
- 対象: ブランチ名と `git diff origin/main...<branch>`
- 背景: 何を直す PR か（Issue の要約）と、変えたファイル
- **具体的な観点を挙げる。** 例:
  - 生成されるコードが、組み合わせ（複数の path パラメータ、body と path、戻り値の種類）でコンパイルできるか
  - 既存の snapshot・生成物との互換性
  - docs が実装と合っているか
  - CI の YAML の正しさ
- 実際に生成して `go vet` するなど、**動かして確かめる**よう頼む
- 結果は「重い順に、file:line と具体的な再現つきで。マージを止めるものがあるか」で、日本語で簡潔に

私の結論（「たぶん大丈夫」など）は渡さない。コードと観点だけを渡す。

追加のコミットを push したら、レビュー中のサブエージェントに SendMessage で知らせ、対象に含めてもらう。

## 2. CI と SonarQube を確かめる

```bash
gh pr checks <N> -R zono0013/codegen --watch --interval 30   # 全部終わるまで待つ
gh pr checks <N> -R zono0013/codegen | grep -v pass           # 落ちたものだけ
.claude/skills/review-merge/scripts/sonar-status.sh <N>
```

- SonarQube の解析（`SonarQube Cloud の解析`）は CI より後に終わることがある。`sonar-status.sh` が `Component ... not found` になるなら、まだ終わっていない
- **Quality Gate が落ちたら、まず新しいコードのカバレッジを見る**（しきい値 80%）。`sonar-status.sh` が、テストが通っていない行のあるファイルを出す。除外せず、テストを足して直す
  - Go のカバレッジはパッケージごと（`go test -coverprofile`）。あるパッケージのコードを別のパッケージのテストからしか通していないと、0% になる。そのパッケージ自身にテストを書く
- 指摘（issue / ホットスポット）は直す。誤検知なら、SonarQube の画面で False Positive にするようユーザーに頼み、PR に理由を書く

## 3. 指摘に対応する

- マージを止める指摘は必ず直す。それ以外も、直すのが安いものは直す
- 直したら追加のコミットで push し、PR に**対応のコメント**を書く。指摘の番号ごとに「直した（どう直したか）」「見送った（理由）」を書く
- 指摘が既存の問題（この PR が原因ではない）なら、この PR では直さず Issue を立てる（`issue`）
- 直したら、もう一度 CI と SonarQube を確かめる。大きく直したなら、同じサブエージェントに再レビューを頼む

## 4. マージする

条件（すべて満たしたら、ユーザーに聞かずにマージしてよい）:

- [ ] サブエージェントのレビューに、マージを止める指摘が残っていない
- [ ] CI の必須チェックがすべて `pass`
- [ ] SonarQube の Quality Gate が `OK`、未解決の指摘が 0 件

```bash
gh pr merge <N> -R zono0013/codegen --squash --delete-branch
gh issue view <Issue番号> -R zono0013/codegen --json state -q .state   # Closes #N で CLOSED になったか
git switch main && git pull --ff-only
```

- Issue が閉じていなければ、PR へのリンクをコメントして閉じる
- 後に続く PR（同じファイルを触るもの）があれば、ここで rebase して push する

## Renovate の PR

- minor / patch: CI と SonarQube が通っていればマージしてよい。サブエージェントのレビューは、変更履歴（リリースノート）に互換性を壊す変更が無いかを見てもらう
- major（GitHub Actions の `v6 → v7` など）: リリースノートで破壊的変更を確かめる。ワークフローの入力やランタイム（Node のバージョン）の変更があれば、CI の該当ジョブが実際に動いたかも見る
- 脆弱性の修正（major 以外）は、`renovate.json` の設定で、CI が通れば自動でマージされる。手で触らない
- `mergeable` が `UNKNOWN` のときは Renovate が rebase 中。少し待ってから見る
