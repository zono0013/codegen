# Design Tokens

## カラーパレット

```
--color-base:        #0D0F12   背景
--color-surface:     #161A20   カード・パネル
--color-surface-alt: #1E2329   浮き上がった要素
--color-border:      #2C333D   区切り線・ボーダー
--color-text-sub:    #8B949E   補助テキスト・ラベル
--color-text:        #E6EDF3   本文テキスト
--color-amber:       #F0B72F   アクセント・財務数値強調
--color-green:       #3FB950   進捗良好・黒字・成功
--color-red:         #F85149   遅延・赤字・エラー
```

## タイポグラフィ

- **UI テキスト**: Geist Sans — ナビゲーション・ラベル・本文
- **数値**: Geist Mono — 財務金額・パーセンテージ・日付すべてに適用

## シグネチャ：アンバー Mono 数値

このプロダクトの視覚的アイデンティティ。**財務数値はすべて `color-amber` + Geist Mono で表示する。** テーブルの中だけでなく、文章中のインライン数値にも適用する。

```
収益  ¥12.4M / ¥15.0M   残 ¥2.6M
      ^^^^^^   ^^^^^^       ^^^^^^  ← amber + mono
```

## Tailwind への対応

```js
// tailwind.config.ts
colors: {
  base:        '#0D0F12',
  surface:     '#161A20',
  'surface-alt':'#1E2329',
  border:      '#2C333D',
  'text-sub':  '#8B949E',
  text:        '#E6EDF3',
  amber:       '#F0B72F',
  green:       '#3FB950',
  red:         '#F85149',
}
```

財務数値には `font-mono text-amber` を必ず付与する。
