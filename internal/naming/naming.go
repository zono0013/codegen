// Package naming は生成物の識別子・キー名の変換規則。
// DTO の JSON キー（rest）と生成 Go コード（gogen）が同じ規則で揃っている必要があるため 1 箇所に置く。
package naming

import (
	"strings"

	"github.com/jinzhu/inflection"
)

// ToSnake は Go の名前を snake_case にする（"CreatedAt" → "created_at"、"HTTPLog" → "http_log"）。
// 大文字の並び（頭字語）は 1 語として扱う（GORM の既定の命名と同じ）。message 名・service 名（テーブル名・ファイル名）に使う。
// フィールドの列名・JSON のキーは proto のフィールド名をそのまま使うので、これは通さない。
func ToSnake(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if isUpper(c) {
			// 小文字・数字の後、または大文字の並びの最後（次が小文字）で区切る（"HTTPLog" の "L" の前）
			if i > 0 && (!isUpper(s[i-1]) || i+1 < len(s) && isLower(s[i+1])) && s[i-1] != '_' {
				b.WriteByte('_')
			}
			c += 'a' - 'A'
		}
		b.WriteByte(c)
	}
	return b.String()
}

func isUpper(c byte) bool { return c >= 'A' && c <= 'Z' }
func isLower(c byte) bool { return c >= 'a' && c <= 'z' }

// Plural は英語の複数形（"community" → "communities"、"person" → "people"）。語尾の単語だけを変える。
// 複数形の規則は、GORM の既定の命名（schema.NamingStrategy）と同じ inflection に揃える。
func Plural(s string) string {
	return inflection.Plural(s)
}
