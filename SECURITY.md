# セキュリティポリシー

## 対応する版

`main` ブランチの最新だけを対象にします。codegen を Submodule として使っている場合は、Submodule を `main` の最新に上げてください。

## 脆弱性の報告

**公開の Issue や Pull Request には書かないでください。** GitHub の Private vulnerability reporting で非公開に報告してください。

1. [Security タブの「Report a vulnerability」](https://github.com/zono0013/codegen/security/advisories/new) を開く
2. 次を書く
   - 影響を受けるもの（ジェネレータ本体、生成されるコード、`example/` のどれか。生成されるコードなら、どのテンプレートやランタイムか）
   - 再現の手順（proto や `codegen.yaml` の例があると助かります）
   - 想定される影響

内容を確認して返信します。修正したら、報告者と相談のうえで GitHub Security Advisory として公開します。

## このリポジトリでの対策

- 依存の脆弱性: Renovate による更新と、CI での `govulncheck` / `pnpm audit`（毎日も実行）
- GitHub Actions: アクションとコンテナイメージをハッシュで固定し、zizmor でワークフローを検査
- コード: SonarQube Cloud による静的解析
