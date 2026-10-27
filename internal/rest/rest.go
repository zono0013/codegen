// Package rest は生成される REST Handler の振る舞い（どのフィールドをどこから読み、何をどのステータスで返すか）。
// Handler（gogen）・FE の型（tsgen）・OpenAPI（openapigen）が食い違わないよう、全員がこの規則を使う。
package rest

import (
	"net/http"
	"slices"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/zono0013/codegen/internal/annotation"
)

// IsEntity は @entity が付き、かつ @pk フィールドを持つ message かを返す。
// @pk がない @entity は gogen が Entity を生成しないため entity 扱いしない。
func IsEntity(m *protogen.Message) bool {
	if m == nil || !annotation.HasMarker(string(m.Comments.Leading), "@entity") {
		return false
	}
	return slices.ContainsFunc(m.Fields, func(f *protogen.Field) bool {
		return annotation.HasMarker(string(f.Comments.Leading), "@pk")
	})
}

type ShapeKind int

const (
	ShapeEmpty  ShapeKind = iota // フィールド 0 個 → 204 No Content（body なし）
	ShapeEntity                  // @entity 1 個（単数） → その DTO
	ShapeList                    // @entity 1 個（repeated） → DTO の配列
	ShapeOutput                  // それ以外 → 全フィールドを持つオブジェクト
)

// Shape は Handler が実際に JSON に書き出す Response の形。
// Handler は Response メッセージをそのまま返さず「中身」を返す。
type Shape struct {
	Kind   ShapeKind
	Entity *protogen.Message // ShapeEntity / ShapeList のときの entity
}

func ResponseShape(res *protogen.Message) Shape {
	if len(res.Fields) == 0 {
		return Shape{Kind: ShapeEmpty}
	}
	if len(res.Fields) == 1 {
		f := res.Fields[0]
		if f.Message != nil && !f.Desc.IsMap() && IsEntity(f.Message) {
			if f.Desc.IsList() {
				return Shape{Kind: ShapeList, Entity: f.Message}
			}
			return Shape{Kind: ShapeEntity, Entity: f.Message}
		}
	}
	return Shape{Kind: ShapeOutput}
}

// SuccessStatus は成功時の HTTP ステータス。
func SuccessStatus(method string, s Shape) int {
	switch {
	case s.Kind == ShapeEmpty:
		return http.StatusNoContent
	case method == http.MethodPost:
		return http.StatusCreated
	default:
		return http.StatusOK
	}
}

// Fields は Request メッセージのフィールドを読み出し元ごとに分けたもの。
type Fields struct {
	Path  []*protogen.Field // URL の {name}
	Query []*protogen.Field // GET / DELETE で query string から読む scalar
	Body  []*protogen.Field // POST / PUT / PATCH の JSON body
}

// IsQueryMethod は path 以外のフィールドを body ではなく query string から読むメソッドか。
func IsQueryMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodDelete
}

func Partition(req *protogen.Message, h *annotation.HTTP) Fields {
	pathParams := annotation.PathParams(h.Path)
	isQueryMethod := IsQueryMethod(h.Method)
	var out Fields
	for _, f := range req.Fields {
		switch {
		case slices.Contains(pathParams, string(f.Desc.Name())):
			out.Path = append(out.Path, f)
		case isQueryMethod:
			// query string から読めない型（repeated / map / message 等）は Handler が無視する
			if IsQueryable(f) {
				out.Query = append(out.Query, f)
			}
		default:
			out.Body = append(out.Body, f)
		}
	}
	return out
}

// IsQueryable は Handler が query string から値を読めるフィールドかを返す。
func IsQueryable(f *protogen.Field) bool {
	if f.Desc.IsList() || f.Desc.IsMap() {
		return false
	}
	switch f.Desc.Kind() {
	case protoreflect.StringKind, protoreflect.BoolKind,
		protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind,
		protoreflect.Uint32Kind, protoreflect.Fixed32Kind,
		protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind,
		protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return true
	}
	return false
}

// IsHidden は API に出さない entity のフィールド（@hidden）かを返す。DB の列（entity）には残し、
// DTO・TS・OpenAPI からは外す。
func IsHidden(f *protogen.Field) bool {
	return annotation.HasMarker(string(f.Comments.Leading), "@hidden")
}

// IsTimestamp は日時のフィールド（google.protobuf.Timestamp）かを返す。生成物では time.Time、JSON では RFC 3339 の文字列。
// map は value 側を渡す。
func IsTimestamp(f *protogen.Field) bool {
	return !f.Desc.IsMap() && f.Desc.Kind() == protoreflect.MessageKind && f.Desc.Message().FullName() == "google.protobuf.Timestamp"
}
