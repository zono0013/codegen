
package entity

import (
	"errors"
	"strings"
	"time"
{{- if .HasDecimal}}

	"github.com/shopspring/decimal"
{{- end}}
)

type {{.Name}} struct {
{{- range .Fields}}
	{{.GoName}} {{.GoType}} `gorm:"{{gormTag .}}"`
{{- end}}
}

func ({{.Name}}) TableName() string { return "{{.SnakePlural}}" }

func New{{.Name}}(
{{- range .Fields}}
	{{.ParamName}} {{.GoType}},
{{- end}}
) (*{{.Name}}, error) {
{{- range .Fields}}
{{- if and .IsPK (eq .GoType "string")}}
	if {{.ParamName}} == "" {
		return nil, errors.New("{{.SnakeName}} is required")
	}
{{- end}}
{{- if and .IsRequired (eq .GoType "string")}}
	{{.ParamName}} = strings.TrimSpace({{.ParamName}})
	if {{.ParamName}} == "" {
		return nil, errors.New("{{.SnakeName}} is required")
	}
{{- end}}
{{- if .IsEmail}}
	{{.ParamName}} = strings.TrimSpace({{.ParamName}})
	if {{.ParamName}} == "" || !strings.Contains({{.ParamName}}, "@") {
		return nil, errors.New("{{.SnakeName}} is invalid")
	}
{{- end}}
{{- end}}
	return &{{.Name}}{
{{- range .Fields}}
		{{.GoName}}: {{.ParamName}},
{{- end}}
	}, nil
}

// 検査の無い entity でも import を使うため（どのフィールドが検査を持つかで import を変えない）
var (
	_ = time.Time{}
	_ = errors.New
	_ = strings.TrimSpace
)
{{- if .HasDecimal}}

var _ = decimal.Decimal{}
{{- end}}
