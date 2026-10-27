
package handler

import (
	"encoding/json"
	"net/http"
{{- if .NeedsStrconv}}
	"strconv"
{{- end}}
{{- if .HandlerNeedsTime}}
	"time"
{{- end}}

	"{{importPath}}/pkg/util/errs"
	"{{importPath}}/internal/usecase"
)

type {{.HandlerTypeName}} struct {
	usecase   usecase.{{.UsecaseTypeName}}
	errWriter errs.ErrorWriter
}

func New{{.HandlerTypeName}}(errWriter errs.ErrorWriter, uc usecase.{{.UsecaseTypeName}}) *{{.HandlerTypeName}} {
	return &{{.HandlerTypeName}}{errWriter: errWriter, usecase: uc}
}
{{define "input" -}}
usecase.{{.InputTypeName}}{
{{- range .PathParamFields}}
		{{.GoName}}: {{if .ParseFormat}}{{template "parsed" .}}{{else}}r.PathValue("{{.JsonName}}"){{end}},
{{- end}}
{{- range .BodyFields}}
		{{.GoName}}: body.{{.GoName}},
{{- end}}
{{- range .QueryFields}}
		{{.GoName}}: {{template "parsed" .}},
{{- end}}
	}
{{- end}}
{{define "parsed"}}{{if .ParsedConv}}{{.ParsedConv}}(_{{.ParamName}}){{else}}_{{.ParamName}}{{end}}{{end}}
{{- range .Methods}}
{{- if .Http}}
{{- if .BodyFields}}

type {{.Name}}Body struct {
{{- range .BodyFields}}
	{{.GoName}} {{.HandlerGoType}} `json:"{{.JsonName}}"`
{{- end}}
}
{{- end}}

// {{.Http.Method}} {{.Http.Path}}
func (h *{{$.HandlerTypeName}}) {{.Name}}(w http.ResponseWriter, r *http.Request) {
{{- if .BodyFields}}
	var body {{.Name}}Body
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		h.errWriter.WriteError(w, r, errs.NewBadRequestError("invalid request body"))
		return
	}
{{- end}}
{{- range .PathParamFields}}
{{- if .ParseFormat}}
	_{{.ParamName}}, err := {{printf .ParseFormat (printf "r.PathValue(%q)" .JsonName)}}
	if err != nil {
		h.errWriter.WriteError(w, r, errs.NewBadRequestError("invalid path parameter: {{.JsonName}}"))
		return
	}
{{- end}}
{{- end}}
{{- if .QueryFields}}
	urlQuery := r.URL.Query()
{{- range .QueryFields}}
{{- if eq .ScalarGoType "string"}}
	_{{.ParamName}} := urlQuery.Get("{{.JsonName}}")
{{- else}}
{{- /* 無い・読めない query はゼロ値（範囲外の整数は strconv が返す端の値）として扱う。query は省略できるので 400 にしない */}}
	_{{.ParamName}}, _ := {{printf .ParseFormat (printf "urlQuery.Get(%q)" .JsonName)}}
{{- end}}
{{- end}}
{{- end}}
{{- if .ReturnsEntity}}
	result, err := h.usecase.{{.Name}}(r.Context(), {{template "input" .}})
	if err != nil {
		h.errWriter.WriteError(w, r, err)
		return
	}
	if result == nil {
		h.errWriter.WriteError(w, r, errs.New(errs.ErrorTypeNotFound, "{{.EntityLowerFirst}} not found"))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader({{.Status}})
	_ = json.NewEncoder(w).Encode(result)
{{- else if .ReturnsList}}
	list, err := h.usecase.{{.Name}}(r.Context(), {{template "input" .}})
	if err != nil {
		h.errWriter.WriteError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader({{.Status}})
	_ = json.NewEncoder(w).Encode(list)
{{- else if .ReturnsOutput}}
	result, err := h.usecase.{{.Name}}(r.Context(), {{template "input" .}})
	if err != nil {
		h.errWriter.WriteError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader({{.Status}})
	_ = json.NewEncoder(w).Encode(result)
{{- else}}
	if err := h.usecase.{{.Name}}(r.Context(), {{template "input" .}}); err != nil {
		h.errWriter.WriteError(w, r, err)
		return
	}
	w.WriteHeader({{.Status}})
{{- end}}
}
{{- end}}
{{end}}
