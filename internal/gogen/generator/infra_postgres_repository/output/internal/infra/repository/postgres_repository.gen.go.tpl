
package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"{{importPath}}/internal/domain/entity"
	"{{importPath}}/internal/domain/repository"
	"{{importPath}}/pkg/util/errs"
	"{{importPath}}/pkg/util/tx"
)

type postgres{{.Name}}Repository struct {
	rdbWrapper errs.RDBErrorWrapper
}

func NewPostgres{{.Name}}Repository(rdbWrapper errs.RDBErrorWrapper) repository.{{.Name}}Repository {
	return &postgres{{.Name}}Repository{rdbWrapper: rdbWrapper}
}

// AutoMigrate{{.Name}} は開発用途のスキーマ同期ヘルパー。
// 本番では Goose / Atlas / Flyway 等のマイグレーションツールを使うこと。
func AutoMigrate{{.Name}}(db *gorm.DB) error {
	return db.AutoMigrate(&entity.{{.Name}}{})
}

var _ repository.{{.Name}}Repository = (*postgres{{.Name}}Repository)(nil)
var _ = time.Time{}

func (r *postgres{{.Name}}Repository) SelectAll(ctx context.Context) ([]*entity.{{.Name}}, error) {
	var out []*entity.{{.Name}}
	if err := tx.From(ctx).WithContext(ctx).Find(&out).Error; err != nil {
		return nil, r.rdbWrapper.Wrap("{{.SnakeName}}", err)
	}
	return out, nil
}

func (r *postgres{{.Name}}Repository) SelectByPK(ctx context.Context, {{.PKField.ParamName}} {{.PKField.GoType}}) (*entity.{{.Name}}, error) {
	var e entity.{{.Name}}
	err := tx.From(ctx).WithContext(ctx).Where("{{.PKField.SnakeName}} = ?", {{.PKField.ParamName}}).First(&e).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, r.rdbWrapper.Wrap("{{.SnakeName}}", err)
	}
	return &e, nil
}
{{range .UniqueFieldsNonPK}}
func (r *postgres{{$.Name}}Repository) SelectBy{{.GoName}}(ctx context.Context, {{.ParamName}} {{.GoType}}) (*entity.{{$.Name}}, error) {
	var e entity.{{$.Name}}
	err := tx.From(ctx).WithContext(ctx).Where("{{.SnakeName}} = ?", {{.ParamName}}).First(&e).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, r.rdbWrapper.Wrap("{{$.SnakeName}}", err)
	}
	return &e, nil
}
{{end}}
func (r *postgres{{.Name}}Repository) Insert(ctx context.Context, {{.LowerFirst}} *entity.{{.Name}}) error {
	if err := tx.From(ctx).WithContext(ctx).Create({{.LowerFirst}}).Error; err != nil {
		return r.rdbWrapper.Wrap("{{.SnakeName}}", err)
	}
	return nil
}

func (r *postgres{{.Name}}Repository) BulkInsert(ctx context.Context, {{.Plural}} []*entity.{{.Name}}) error {
	if len({{.Plural}}) == 0 {
		return nil
	}
	if err := tx.From(ctx).WithContext(ctx).CreateInBatches({{.Plural}}, 100).Error; err != nil {
		return r.rdbWrapper.Wrap("{{.SnakeName}}", err)
	}
	return nil
}

func (r *postgres{{.Name}}Repository) Upsert(ctx context.Context, {{.LowerFirst}} *entity.{{.Name}}) error {
	return r.rdbWrapper.Wrap("{{.SnakeName}}", tx.From(ctx).WithContext(ctx).Clauses(clause.OnConflict{UpdateAll: true}).Create({{.LowerFirst}}).Error)
}

func (r *postgres{{.Name}}Repository) BulkUpsert(ctx context.Context, {{.Plural}} []*entity.{{.Name}}) error {
	if len({{.Plural}}) == 0 {
		return nil
	}
	return r.rdbWrapper.Wrap("{{.SnakeName}}", tx.From(ctx).WithContext(ctx).Clauses(clause.OnConflict{UpdateAll: true}).CreateInBatches({{.Plural}}, 100).Error)
}

func (r *postgres{{.Name}}Repository) Update(ctx context.Context, {{.LowerFirst}} *entity.{{.Name}}) error {
	// Select("*") がないと GORM は struct のゼロ値フィールド（false / 0 / ""）を
	// SET 句から落とすため、bool を false に戻す更新が黙って無視される。
	res := tx.From(ctx).WithContext(ctx).Model(&entity.{{.Name}}{}).Where("{{.PKField.SnakeName}} = ?", {{.LowerFirst}}.{{.PKField.GoName}}).Select("*").Updates({{.LowerFirst}})
	if res.Error != nil {
		return r.rdbWrapper.Wrap("{{.SnakeName}}", res.Error)
	}
	if res.RowsAffected == 0 {
		return errs.New(errs.ErrorTypeNotFound, "{{.SnakeName}} not found")
	}
	return nil
}

func (r *postgres{{.Name}}Repository) Delete(ctx context.Context, {{.PKField.ParamName}} {{.PKField.GoType}}) error {
	res := tx.From(ctx).WithContext(ctx).Where("{{.PKField.SnakeName}} = ?", {{.PKField.ParamName}}).Delete(&entity.{{.Name}}{})
	if res.Error != nil {
		return r.rdbWrapper.Wrap("{{.SnakeName}}", res.Error)
	}
	if res.RowsAffected == 0 {
		return errs.New(errs.ErrorTypeNotFound, "{{.SnakeName}} not found")
	}
	return nil
}

func (r *postgres{{.Name}}Repository) BulkDelete(ctx context.Context, {{.PKField.ParamName}}s []{{.PKField.GoType}}) error {
	if len({{.PKField.ParamName}}s) == 0 {
		return nil
	}
	res := tx.From(ctx).WithContext(ctx).Where("{{.PKField.SnakeName}} IN ?", {{.PKField.ParamName}}s).Delete(&entity.{{.Name}}{})
	if res.Error != nil {
		return r.rdbWrapper.Wrap("{{.SnakeName}}", res.Error)
	}
	if res.RowsAffected != int64(len({{.PKField.ParamName}}s)) {
		return errs.New(errs.ErrorTypeNotFound, "{{.SnakeName}} not found")
	}
	return nil
}

func (r *postgres{{.Name}}Repository) DeleteAll(ctx context.Context) error {
	return r.rdbWrapper.Wrap("{{.SnakeName}}", tx.From(ctx).WithContext(ctx).Where("1 = 1").Delete(&entity.{{.Name}}{}).Error)
}
{{- if .PagingField}}

func (r *postgres{{.Name}}Repository) SelectByCursor(ctx context.Context, limit int, after *{{.PagingField.GoType}}) ([]*entity.{{.Name}}, error) {
	q := tx.From(ctx).WithContext(ctx).Order("{{.PagingField.SnakeName}} ASC").Limit(limit)
	if after != nil {
		q = q.Where("{{.PagingField.SnakeName}} > ?", *after)
	}
	var out []*entity.{{.Name}}
	if err := q.Find(&out).Error; err != nil {
		return nil, r.rdbWrapper.Wrap("{{.SnakeName}}", err)
	}
	return out, nil
}
{{- end}}