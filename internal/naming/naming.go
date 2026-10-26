// Package naming は生成物の識別子・キー名の変換規則。
// DTO の JSON キー（rest）と生成 Go コード（gogen）が同じ規則で揃っている必要があるため 1 箇所に置く。
package naming

import (
	"strings"

	"github.com/jinzhu/inflection"
)

// ToSnake は "CreatedAt" → "created_at"。大文字の直前で区切るだけなので "URL" は "u_r_l" になる。
func ToSnake(s string) string {
	var b strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			b.WriteByte('_')
		}
		if r >= 'A' && r <= 'Z' {
			b.WriteRune(r + 32)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Plural は英語の複数形（"community" → "communities"、"person" → "people"）。語尾の単語だけを変える。
// 複数形の規則は、GORM の既定の命名（schema.NamingStrategy）と同じ inflection に揃える。
func Plural(s string) string {
	return inflection.Plural(s)
}
