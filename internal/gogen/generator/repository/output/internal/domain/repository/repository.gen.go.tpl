
package repository

import (
	"context"

	"{{importPath}}/internal/domain/entity"
)

type {{.Name}}Repository interface {
	SelectAll(ctx context.Context) ([]*entity.{{.Name}}, error)
	SelectByPK(ctx context.Context, {{.PKField.ParamName}} {{.PKField.GoType}}) (*entity.{{.Name}}, error)
{{- range .UniqueFieldsNonPK}}
	SelectBy{{.GoName}}(ctx context.Context, {{.ParamName}} {{.GoType}}) (*entity.{{$.Name}}, error)
{{- end}}
	Insert(ctx context.Context, {{.LowerFirst}} *entity.{{.Name}}) error
	BulkInsert(ctx context.Context, {{.Plural}} []*entity.{{.Name}}) error
	Upsert(ctx context.Context, {{.LowerFirst}} *entity.{{.Name}}) error
	BulkUpsert(ctx context.Context, {{.Plural}} []*entity.{{.Name}}) error
	Update(ctx context.Context, {{.LowerFirst}} *entity.{{.Name}}) error
	Delete(ctx context.Context, {{.PKField.ParamName}} {{.PKField.GoType}}) error
	BulkDelete(ctx context.Context, {{.PKField.ParamName}}s []{{.PKField.GoType}}) error
	DeleteAll(ctx context.Context) error
{{- if .PagingField}}
	SelectByCursor(ctx context.Context, limit int, after *{{.PagingField.GoType}}) ([]*entity.{{.Name}}, error)
{{- end}}
}
