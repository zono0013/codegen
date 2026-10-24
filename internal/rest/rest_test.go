package rest

import (
	"slices"
	"testing"

	"google.golang.org/protobuf/compiler/protogen"

	"github.com/zono0013/codegen/internal/annotation"
	"github.com/zono0013/codegen/internal/prototest"
)

const testProto = `syntax = "proto3";
package t.v1;
// @entity
message Item {
  // @pk
  string id = 1;
  string name = 2;
}

// @pk がないので entity 扱いしない（gogen と同じ規則）
// @entity
message NoPK {
  string id = 1;
}

message Summary {
  int64 total = 1;
}

message Empty {}
message One { Item item = 1; }
message Many { repeated Item items = 1; }
message Multi { Item item = 1; int64 total = 2; }
message NonEntity { Summary summary = 1; }
message ByKey { map<string, Item> items = 1; }
message NoPKOne { NoPK x = 1; }

message GetReq {
  string id = 1;
  int32 limit = 2;
  bool verbose = 3;
  repeated string tags = 4;
  Summary filter = 5;
}
message UpdateReq {
  string id = 1;
  string name = 2;
}
`

func messages(t *testing.T) map[string]*protogen.Message {
	t.Helper()
	files := prototest.Files(t, map[string]string{"t/v1/t.proto": testProto})
	out := map[string]*protogen.Message{}
	for _, m := range files[0].Messages {
		out[string(m.Desc.Name())] = m
	}
	return out
}

func TestIsEntity(t *testing.T) {
	msgs := messages(t)
	tests := []struct {
		msg  string
		want bool
	}{
		{"Item", true},
		{"NoPK", false},
		{"Summary", false},
	}
	for _, tt := range tests {
		t.Run(tt.msg, func(t *testing.T) {
			if got := IsEntity(msgs[tt.msg]); got != tt.want {
				t.Errorf("IsEntity(%s) = %v, want %v", tt.msg, got, tt.want)
			}
		})
	}
}

func TestResponseShape(t *testing.T) {
	msgs := messages(t)
	tests := []struct {
		msg        string
		wantKind   ShapeKind
		wantEntity string
	}{
		{"Empty", ShapeEmpty, ""},
		{"One", ShapeEntity, "Item"},
		{"Many", ShapeList, "Item"},
		{"Multi", ShapeOutput, ""},
		{"NonEntity", ShapeOutput, ""},
		{"ByKey", ShapeOutput, ""},
		{"NoPKOne", ShapeOutput, ""},
	}
	for _, tt := range tests {
		t.Run(tt.msg, func(t *testing.T) {
			got := ResponseShape(msgs[tt.msg])
			if got.Kind != tt.wantKind {
				t.Errorf("Kind = %v, want %v", got.Kind, tt.wantKind)
			}
			gotEntity := ""
			if got.Entity != nil {
				gotEntity = string(got.Entity.Desc.Name())
			}
			if gotEntity != tt.wantEntity {
				t.Errorf("Entity = %q, want %q", gotEntity, tt.wantEntity)
			}
		})
	}
}

func TestSuccessStatus(t *testing.T) {
	tests := []struct {
		name   string
		method string
		kind   ShapeKind
		want   int
	}{
		{"POST で作成は 201", "POST", ShapeEntity, 201},
		{"POST で Output も 201", "POST", ShapeOutput, 201},
		{"GET は 200", "GET", ShapeList, 200},
		{"PATCH は 200", "PATCH", ShapeEntity, 200},
		{"空レスポンスは 204", "DELETE", ShapeEmpty, 204},
		{"POST でも空なら 204", "POST", ShapeEmpty, 204},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SuccessStatus(tt.method, Shape{Kind: tt.kind}); got != tt.want {
				t.Errorf("SuccessStatus = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestPartition(t *testing.T) {
	msgs := messages(t)
	names := func(fs []*protogen.Field) []string {
		var out []string
		for _, f := range fs {
			out = append(out, string(f.Desc.Name()))
		}
		return out
	}
	tests := []struct {
		name                          string
		req                           string
		http                          annotation.HTTP
		wantPath, wantQuery, wantBody []string
	}{
		{
			name:      "GET は path 以外の scalar を query にする（repeated / message は読めないので除外）",
			req:       "GetReq",
			http:      annotation.HTTP{Method: "GET", Path: "/items/{id}"},
			wantPath:  []string{"id"},
			wantQuery: []string{"limit", "verbose"},
		},
		{
			name:     "PATCH は path 以外を body にする",
			req:      "UpdateReq",
			http:     annotation.HTTP{Method: "PATCH", Path: "/items/{id}"},
			wantPath: []string{"id"},
			wantBody: []string{"name"},
		},
		{
			name:     "POST で path param なしなら全部 body",
			req:      "UpdateReq",
			http:     annotation.HTTP{Method: "POST", Path: "/items"},
			wantBody: []string{"id", "name"},
		},
		{
			name:     "DELETE は path だけなら query なし",
			req:      "UpdateReq",
			http:     annotation.HTTP{Method: "DELETE", Path: "/items/{id}/{name}"},
			wantPath: []string{"id", "name"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Partition(msgs[tt.req], &tt.http)
			if g := names(got.Path); !slices.Equal(g, tt.wantPath) {
				t.Errorf("Path = %v, want %v", g, tt.wantPath)
			}
			if g := names(got.Query); !slices.Equal(g, tt.wantQuery) {
				t.Errorf("Query = %v, want %v", g, tt.wantQuery)
			}
			if g := names(got.Body); !slices.Equal(g, tt.wantBody) {
				t.Errorf("Body = %v, want %v", g, tt.wantBody)
			}
		})
	}
}

func TestDTOFieldName(t *testing.T) {
	files := prototest.Files(t, map[string]string{"d/v1/d.proto": `syntax = "proto3";
package d.v1;
message M {
  string author_id = 1;
  string authorName = 2;
  // @timestamp
  int64 created_at_unix = 3;
}
`})
	var got []string
	for _, f := range files[0].Messages[0].Fields {
		got = append(got, DTOFieldName(f))
	}
	if want := []string{"author_id", "author_name", "created_at_unix"}; !slices.Equal(got, want) {
		t.Errorf("DTOFieldName = %v, want %v", got, want)
	}
}
