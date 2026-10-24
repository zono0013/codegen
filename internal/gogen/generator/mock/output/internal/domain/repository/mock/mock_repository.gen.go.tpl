
package mock

import (
	"context"

	"{{importPath}}/internal/domain/entity"
	"{{importPath}}/internal/domain/repository"
)

// Mock{{.Name}}Repository は repository.{{.Name}}Repository のテスト用スタブ。
// 各 Func フィールドにスタブ関数を差し込み、未設定ならゼロ値を返す。
type Mock{{.Name}}Repository struct {
	SelectAllFunc func(ctx context.Context) ([]*entity.{{.Name}}, error)
	SelectByPKFunc func(ctx context.Context, {{.PKField.ParamName}} {{.PKField.GoType}}) (*entity.{{.Name}}, error)
{{- range .UniqueFieldsNonPK}}
	SelectBy{{.GoName}}Func func(ctx context.Context, {{.ParamName}} {{.GoType}}) (*entity.{{$.Name}}, error)
{{- end}}
	InsertFunc func(ctx context.Context, {{.LowerFirst}} *entity.{{.Name}}) error
	BulkInsertFunc func(ctx context.Context, {{.Plural}} []*entity.{{.Name}}) error
	UpsertFunc func(ctx context.Context, {{.LowerFirst}} *entity.{{.Name}}) error
	BulkUpsertFunc func(ctx context.Context, {{.Plural}} []*entity.{{.Name}}) error
	UpdateFunc func(ctx context.Context, {{.LowerFirst}} *entity.{{.Name}}) error
	DeleteFunc func(ctx context.Context, {{.PKField.ParamName}} {{.PKField.GoType}}) error
	BulkDeleteFunc func(ctx context.Context, {{.PKField.ParamName}}s []{{.PKField.GoType}}) error
	DeleteAllFunc func(ctx context.Context) error
{{- if .PagingField}}
	SelectByCursorFunc func(ctx context.Context, limit int, after *{{.PagingField.GoType}}) ([]*entity.{{.Name}}, error)
{{- end}}
}

var _ repository.{{.Name}}Repository = (*Mock{{.Name}}Repository)(nil)

func (m *Mock{{.Name}}Repository) SelectAll(ctx context.Context) ([]*entity.{{.Name}}, error) {
	if m.SelectAllFunc != nil {
		return m.SelectAllFunc(ctx)
	}
	return nil, nil
}

func (m *Mock{{.Name}}Repository) SelectByPK(ctx context.Context, {{.PKField.ParamName}} {{.PKField.GoType}}) (*entity.{{.Name}}, error) {
	if m.SelectByPKFunc != nil {
		return m.SelectByPKFunc(ctx, {{.PKField.ParamName}})
	}
	return nil, nil
}
{{range .UniqueFieldsNonPK}}
func (m *Mock{{$.Name}}Repository) SelectBy{{.GoName}}(ctx context.Context, {{.ParamName}} {{.GoType}}) (*entity.{{$.Name}}, error) {
	if m.SelectBy{{.GoName}}Func != nil {
		return m.SelectBy{{.GoName}}Func(ctx, {{.ParamName}})
	}
	return nil, nil
}
{{end}}
func (m *Mock{{.Name}}Repository) Insert(ctx context.Context, {{.LowerFirst}} *entity.{{.Name}}) error {
	if m.InsertFunc != nil {
		return m.InsertFunc(ctx, {{.LowerFirst}})
	}
	return nil
}

func (m *Mock{{.Name}}Repository) BulkInsert(ctx context.Context, {{.Plural}} []*entity.{{.Name}}) error {
	if m.BulkInsertFunc != nil {
		return m.BulkInsertFunc(ctx, {{.Plural}})
	}
	return nil
}

func (m *Mock{{.Name}}Repository) Upsert(ctx context.Context, {{.LowerFirst}} *entity.{{.Name}}) error {
	if m.UpsertFunc != nil {
		return m.UpsertFunc(ctx, {{.LowerFirst}})
	}
	return nil
}

func (m *Mock{{.Name}}Repository) BulkUpsert(ctx context.Context, {{.Plural}} []*entity.{{.Name}}) error {
	if m.BulkUpsertFunc != nil {
		return m.BulkUpsertFunc(ctx, {{.Plural}})
	}
	return nil
}

func (m *Mock{{.Name}}Repository) Update(ctx context.Context, {{.LowerFirst}} *entity.{{.Name}}) error {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(ctx, {{.LowerFirst}})
	}
	return nil
}

func (m *Mock{{.Name}}Repository) Delete(ctx context.Context, {{.PKField.ParamName}} {{.PKField.GoType}}) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, {{.PKField.ParamName}})
	}
	return nil
}

func (m *Mock{{.Name}}Repository) BulkDelete(ctx context.Context, {{.PKField.ParamName}}s []{{.PKField.GoType}}) error {
	if m.BulkDeleteFunc != nil {
		return m.BulkDeleteFunc(ctx, {{.PKField.ParamName}}s)
	}
	return nil
}

func (m *Mock{{.Name}}Repository) DeleteAll(ctx context.Context) error {
	if m.DeleteAllFunc != nil {
		return m.DeleteAllFunc(ctx)
	}
	return nil
}
{{- if .PagingField}}

func (m *Mock{{.Name}}Repository) SelectByCursor(ctx context.Context, limit int, after *{{.PagingField.GoType}}) ([]*entity.{{.Name}}, error) {
	if m.SelectByCursorFunc != nil {
		return m.SelectByCursorFunc(ctx, limit, after)
	}
	return nil, nil
}
{{- end}}
