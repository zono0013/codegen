
package usecase

import (
	"context"
{{if .NeedsDTO}}
	"{{importPath}}/internal/dto"
{{end}}
)
{{range .Methods}}
{{- if .InputFields}}

type {{.InputTypeName}} struct {
{{- range .InputFields}}
	{{.GoName}} {{.GoType}}
{{- end}}
}
{{- else}}

type {{.InputTypeName}} struct{}
{{- end}}
{{- end}}
{{range .Methods}}
{{- if .ReturnsOutput}}

type {{.OutputTypeName}} struct {
{{- range .OutputFields}}
	{{.GoName}} {{.GoType}} `json:"{{.JsonTag}}"`
{{- end}}
}
{{- end}}
{{- end}}
{{range .NonEntityTypes}}

type {{.Name}} struct {
{{- range .Fields}}
	{{.GoName}} {{.GoType}} `json:"{{.JsonTag}}"`
{{- end}}
}
{{- end}}

// {{.UsecaseTypeName}} は {{.ServiceName}} の rpc 1 本につき 1 メソッドを持つ interface。
// 実装クラス（<Name>UsecaseImpl）はこの interface を実装する手書き層。
// 戻り値はすべて pkg/dto の DTO 型 / Output 型でクライアント JSON シリアライズ表現を一元管理する。
type {{.UsecaseTypeName}} interface {
{{- range .Methods}}
{{- if .ReturnsEntity}}
	{{.Name}}(ctx context.Context, input {{.InputTypeName}}) (*dto.{{.EntityGoName}}DTO, error)
{{- else if .ReturnsList}}
	{{.Name}}(ctx context.Context, input {{.InputTypeName}}) ([]*dto.{{.EntityGoName}}DTO, error)
{{- else if .ReturnsOutput}}
	{{.Name}}(ctx context.Context, input {{.InputTypeName}}) (*{{.OutputTypeName}}, error)
{{- else}}
	{{.Name}}(ctx context.Context, input {{.InputTypeName}}) error
{{- end}}
{{- end}}
}
