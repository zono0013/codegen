package naming

import "testing"

func TestToSnake(t *testing.T) {
	tests := map[string]string{
		"CreatedAt": "created_at",
		"id":        "id",
		"AuthorId":  "author_id",
		"URL":       "u_r_l",
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
