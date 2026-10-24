package annotation

import (
	"slices"
	"testing"
)

func TestHasMarker(t *testing.T) {
	tests := []struct {
		name    string
		comment string
		marker  string
		want    bool
	}{
		{"単独", " @entity\n", "@entity", true},
		{"同じ行に複数", " @unique @email\n", "@email", true},
		{"説明文の後の行", " User は記事の投稿者。\n\n @entity\n", "@entity", true},
		{"前方一致では拾わない", " @emails\n", "@email", false},
		{"無し", " ただのコメント\n", "@pk", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasMarker(tt.comment, tt.marker); got != tt.want {
				t.Errorf("HasMarker(%q, %q) = %v, want %v", tt.comment, tt.marker, got, tt.want)
			}
		})
	}
}

func TestParseHTTP(t *testing.T) {
	tests := []struct {
		name    string
		comment string
		want    *HTTP
	}{
		{"GET", " @http GET /api/users/{id}\n", &HTTP{Method: "GET", Path: "/api/users/{id}"}},
		{"小文字メソッドは大文字化", " @http patch /api/users/{id}\n", &HTTP{Method: "PATCH", Path: "/api/users/{id}"}},
		{"説明文の後", " ユーザー取得\n @http GET /api/users\n", &HTTP{Method: "GET", Path: "/api/users"}},
		{"未対応メソッドは無視", " @http HEAD /api/users\n", nil},
		{"引数不足は無視", " @http GET\n", nil},
		{"無し", " ユーザー取得\n", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseHTTP(tt.comment)
			if (got == nil) != (tt.want == nil) || (got != nil && *got != *tt.want) {
				t.Errorf("ParseHTTP(%q) = %+v, want %+v", tt.comment, got, tt.want)
			}
		})
	}
}

func TestPathParams(t *testing.T) {
	tests := []struct {
		path string
		want []string
	}{
		{"/api/users", nil},
		{"/api/users/{id}", []string{"id"}},
		{"/api/orgs/{org_id}/users/{id}", []string{"org_id", "id"}},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := PathParams(tt.path); !slices.Equal(got, tt.want) {
				t.Errorf("PathParams(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestParseDecimal(t *testing.T) {
	tests := []struct {
		name      string
		comment   string
		wantP     int
		wantS     int
		wantFound bool
		wantErr   bool
	}{
		{"正常", " @decimal(10,2)\n", 10, 2, true, false},
		{"空白入り", " @decimal( 12 , 4 )\n", 12, 4, true, false},
		{"無し", " @required\n", 0, 0, false, false},
		{"閉じ括弧なし", " @decimal(10,2\n", 0, 0, true, true},
		{"引数が 1 つ", " @decimal(10)\n", 0, 0, true, true},
		{"scale > precision", " @decimal(2,4)\n", 0, 0, true, true},
		{"precision が 0", " @decimal(0,0)\n", 0, 0, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, s, found, err := ParseDecimal(tt.comment)
			if (err != nil) != tt.wantErr || found != tt.wantFound || p != tt.wantP || s != tt.wantS {
				t.Errorf("ParseDecimal(%q) = (%d, %d, %v, %v), want (%d, %d, %v, err=%v)",
					tt.comment, p, s, found, err, tt.wantP, tt.wantS, tt.wantFound, tt.wantErr)
			}
		})
	}
}

func TestDescription(t *testing.T) {
	tests := []struct {
		name    string
		comment string
		want    string
	}{
		{"アノテーション行を除く", " 商品。\n\n @entity\n", "商品。"},
		{"複数行は保つ", " 1 行目。\n 2 行目。\n @pk\n", "1 行目。\n2 行目。"},
		{"アノテーション説明の行も除く", " 記事。\n\n   @entity    Entity を生成\n   @pk        主キー\n", "記事。"},
		{"アノテーションだけなら空", " @pk @paging\n", ""},
		{"空", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Description(tt.comment); got != tt.want {
				t.Errorf("Description(%q) = %q, want %q", tt.comment, got, tt.want)
			}
		})
	}
}
