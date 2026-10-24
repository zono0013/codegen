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
