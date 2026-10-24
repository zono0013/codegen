
package dto

import (
	"time"

	"{{importPath}}/internal/domain/entity"
{{- if .HasDecimal}}
	"github.com/shopspring/decimal"
{{- end}}
)

// {{.Name}}DTO は {{.Name}} エンティティをクライアントへ返すための JSON シリアライズ型。
// json タグはここで一元管理する。entity 側は GORM タグのみ持つ。
type {{.Name}}DTO struct {
{{- range .Fields}}
{{- if .IsTimestamp}}
	{{.GoName}}Unix int64 `json:"{{.SnakeName}}_unix"`
{{- else}}
	{{.GoName}} {{.GoType}} `json:"{{.SnakeName}}"`
{{- end}}
{{- end}}
}

// From{{.Name}} は entity.{{.Name}} を {{.Name}}DTO に変換する。
// nil を渡したら nil を返す。
func From{{.Name}}(e *entity.{{.Name}}) *{{.Name}}DTO {
	if e == nil {
		return nil
	}
	return &{{.Name}}DTO{
{{- range .Fields}}
{{- if .IsTimestamp}}
		{{.GoName}}Unix: e.{{.GoName}}.Unix(),
{{- else}}
		{{.GoName}}: e.{{.GoName}},
{{- end}}
{{- end}}
	}
}

// From{{.Name}}s は []*entity.{{.Name}} を []*{{.Name}}DTO に変換する。
func From{{.Name}}s(es []*entity.{{.Name}}) []*{{.Name}}DTO {
	out := make([]*{{.Name}}DTO, 0, len(es))
	for _, e := range es {
		out = append(out, From{{.Name}}(e))
	}
	return out
}

var _ = time.Time{}
{{- if .HasDecimal}}

var _ = decimal.Decimal{}
{{- end}}
