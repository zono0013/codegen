package naming

import "testing"

func TestToSnake(t *testing.T) {
	// 大文字の並び（頭字語）は 1 語として扱う。GORM の既定の命名と同じ
	tests := map[string]string{
		"CreatedAt":   "created_at",
		"id":          "id",
		"AuthorId":    "author_id",
		"URL":         "url",
		"HTTPLog":     "http_log",
		"UserID":      "user_id",
		"APIKey":      "api_key",
		"ABTest":      "ab_test",
		"MailOutbox":  "mail_outbox",
		"Address2":    "address2",
		"V2Migration": "v2_migration",
	}
	for in, want := range tests {
		if got := ToSnake(in); got != want {
			t.Errorf("ToSnake(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPlural(t *testing.T) {
	// テーブル名（TableName / migration-gen）と、Bulk 系の引数名に使う。GORM の既定の命名と同じ規則
	tests := map[string]string{
		"user":         "users",
		"community":    "communities",
		"category":     "categories",
		"address":      "addresses",
		"box":          "boxes",
		"match":        "matches",
		"person":       "people",
		"day":          "days",
		"tag_category": "tag_categories",
		"tagCategory":  "tagCategories",
		"user_status":  "user_statuses",
		"news":         "news",
	}
	for in, want := range tests {
		if got := Plural(in); got != want {
			t.Errorf("Plural(%q) = %q, want %q", in, got, want)
		}
	}
}
