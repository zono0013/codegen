package tx

import (
	"context"
	"database/sql"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// fakePool は BeginTx に渡された ctx を記録するだけの ConnPool（実 DB を使わずに tx.Run を検証するため）。
type fakePool struct {
	beginCtx context.Context
}

func (p *fakePool) PrepareContext(context.Context, string) (*sql.Stmt, error) { return nil, nil }
func (p *fakePool) ExecContext(context.Context, string, ...any) (sql.Result, error) {
	return nil, nil
}
func (p *fakePool) QueryContext(context.Context, string, ...any) (*sql.Rows, error) {
	return nil, nil
}
func (p *fakePool) QueryRowContext(context.Context, string, ...any) *sql.Row { return nil }

func (p *fakePool) BeginTx(ctx context.Context, _ *sql.TxOptions) (gorm.ConnPool, error) {
	p.beginCtx = ctx
	return p, nil
}
func (p *fakePool) Commit() error   { return nil }
func (p *fakePool) Rollback() error { return nil }

func openFake(t *testing.T) (*gorm.DB, *fakePool) {
	t.Helper()
	pool := &fakePool{}
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: pool}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return db, pool
}

func TestInit_ReplacesDefaultDB(t *testing.T) {
	first, _ := openFake(t)
	second, _ := openFake(t)

	Init(first)
	Init(second)

	if got := From(context.Background()); got != second {
		t.Errorf("2 回目の Init が反映されていない（テストごとに DB を差し替えられない）")
	}
}

type ctxKey struct{}

func TestRun_BeginsWithCallerContext(t *testing.T) {
	db, pool := openFake(t)
	Init(db)

	ctx := context.WithValue(context.Background(), ctxKey{}, "caller")
	if err := Run(ctx, func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}

	if pool.beginCtx == nil || pool.beginCtx.Value(ctxKey{}) != "caller" {
		t.Errorf("BEGIN に呼び出し元の ctx が渡っていない（キャンセル・タイムアウトに追従しない）")
	}
}
