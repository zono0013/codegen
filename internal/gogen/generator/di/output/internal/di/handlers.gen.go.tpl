
package di

import (
	"net/http"

	"{{importPath}}/internal/handler"
	"{{importPath}}/internal/usecase"
	"{{importPath}}/pkg/util/errs"
)

// Handlers は全サービスの REST Handler を束ねた DI コンテナ。
type Handlers struct {
{{- range .Services}}
	{{.ServiceName}} *handler.{{.HandlerTypeName}}
{{- end}}
}

// NewHandlers は各 Usecase を受け取って Handlers を組み立てる。
func NewHandlers(
	errWriter errs.ErrorWriter,
{{- range .Services}}
	{{.UsecaseParamName}} usecase.{{.UsecaseTypeName}},
{{- end}}
) *Handlers {
	return &Handlers{
{{- range .Services}}
		{{.ServiceName}}: handler.New{{.HandlerTypeName}}(errWriter, {{.UsecaseParamName}}),
{{- end}}
	}
}

// Register は全 Handler を ServeMux に登録する（Go 1.22+ の HandleFunc パターン）。
func (h *Handlers) Register(mux *http.ServeMux) {
{{- range .Services}}
{{- $svc := .ServiceName}}
{{- range .Methods}}
{{- if .Http}}
	mux.HandleFunc("{{.Http.Method}} {{.Http.Path}}", h.{{$svc}}.{{.Name}})
{{- end}}
{{- end}}
{{- end}}
}
