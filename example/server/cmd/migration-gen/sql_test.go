package main

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func usersTable() *tableSchema {
	return &tableSchema{
		Name: "users",
		Columns: map[string]columnSchema{
			"id":    {Name: "id", SQLType: "text", PrimaryKey: true, NotNull: true},
			"email": {Name: "email", SQLType: "text", NotNull: true, Unique: true},
		},
	}
}

func TestEmitCreate_NamesUniqueConstraint(t *testing.T) {
	sql := emitSQL(tableChange{Table: "users", Kind: changeCreate, NewTable: usersTable()})
	// 後から外すときと同じ名前にするため、Postgres 既定の名前（users_email_key）に任せない
	if want := `"email" text NOT NULL CONSTRAINT "uq_users_email" UNIQUE`; !strings.Contains(sql, want) {
		t.Errorf("%q が無い:\n%s", want, sql)
	}
}

func TestEmitAlter_DropUniqueTriesBothNames(t *testing.T) {
	sql := emitSQL(tableChange{Table: "users", Kind: changeAlter, DropUniqueCols: []string{"email"}})
	for _, want := range []string{
		`ALTER TABLE "users" DROP CONSTRAINT IF EXISTS "uq_users_email";`,
		// 名前を明示する前に CREATE TABLE で作った unique は、Postgres 既定の名前になっている
		`ALTER TABLE "users" DROP CONSTRAINT IF EXISTS "users_email_key";`,
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("%q が無い:\n%s", want, sql)
		}
	}
}

// TestUniqueConstraintLifecycle_Postgres は、生成した SQL を実際の Postgres に適用して、
// unique を作る → 外す → 足す → 外す が通ることを確かめる。名前を明示する前の CREATE TABLE で作ったテーブルも外せること。
// MIGRATION_TEST_DATABASE_URL が無ければ飛ばす（CI では Postgres を立てて実行する）。
func TestUniqueConstraintLifecycle_Postgres(t *testing.T) {
	url := os.Getenv("MIGRATION_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("MIGRATION_TEST_DATABASE_URL が無い")
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close(ctx) })

	exec := func(sql string) {
		t.Helper()
		if _, err := conn.Exec(ctx, sql); err != nil {
			t.Fatalf("%v\n%s", err, sql)
		}
	}
	uniqueCount := func() int {
		t.Helper()
		var n int
		err := conn.QueryRow(ctx, `SELECT count(*) FROM pg_constraint WHERE conrelid = 'users'::regclass AND contype = 'u'`).Scan(&n)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	reset := func() { exec(`DROP TABLE IF EXISTS "users"`) }
	t.Cleanup(reset)

	t.Run("生成した SQL で作ったテーブル", func(t *testing.T) {
		reset()
		exec(emitSQL(tableChange{Table: "users", Kind: changeCreate, NewTable: usersTable()}))
		exec(emitSQL(tableChange{Table: "users", Kind: changeAlter, DropUniqueCols: []string{"email"}}))
		if n := uniqueCount(); n != 0 {
			t.Fatalf("unique が残っている: %d", n)
		}
		exec(emitSQL(tableChange{Table: "users", Kind: changeAlter, AddUniqueCols: []string{"email"}}))
		exec(emitSQL(tableChange{Table: "users", Kind: changeAlter, DropUniqueCols: []string{"email"}}))
		if n := uniqueCount(); n != 0 {
			t.Fatalf("足した unique が残っている: %d", n)
		}
	})

	t.Run("名前を明示する前の CREATE TABLE で作ったテーブル", func(t *testing.T) {
		reset()
		exec(`CREATE TABLE "users" ("id" text PRIMARY KEY, "email" text NOT NULL UNIQUE)`)
		exec(emitSQL(tableChange{Table: "users", Kind: changeAlter, DropUniqueCols: []string{"email"}}))
		if n := uniqueCount(); n != 0 {
			t.Fatalf("unique が残っている: %d", n)
		}
	})
}
