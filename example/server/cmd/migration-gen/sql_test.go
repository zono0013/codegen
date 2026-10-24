package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

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

func TestEmitAlter_DropUniqueFindsConstraintByColumn(t *testing.T) {
	sql := emitSQL(tableChange{Table: "users", Kind: changeAlter, DropUniqueCols: []string{"email"}})
	// 名前ではなく列で探す（名前を明示する前の既定の名前、手で付けた名前、長い名前の切り詰めでも外せるように）。
	// 見つからなければ失敗させ、snapshot と DB が食い違ったまま進まないようにする
	for _, want := range []string{"pg_constraint", "contype = 'u'", "attname = 'email'", "RAISE EXCEPTION"} {
		if !strings.Contains(sql, want) {
			t.Errorf("%q が無い:\n%s", want, sql)
		}
	}
}

// TestUniqueConstraintLifecycle_Postgres は、生成した SQL を実際の Postgres に適用して、unique を作る・外す・足すが
// 通ることを確かめる。MIGRATION_TEST_DATABASE_URL が無ければ飛ばす（CI では Postgres を立てて実行する）。
// 開発用の DB を指していても既存のテーブルを壊さないよう、テスト専用のスキーマの中で動かし、最後に消す。
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

	exec := func(t *testing.T, sql string) error {
		t.Helper()
		_, err := conn.Exec(ctx, sql)
		return err
	}
	mustExec := func(t *testing.T, sql string) {
		t.Helper()
		if err := exec(t, sql); err != nil {
			t.Fatalf("%v\n%s", err, sql)
		}
	}
	schema := fmt.Sprintf("migration_gen_test_%d", time.Now().UnixNano())
	mustExec(t, fmt.Sprintf(`CREATE SCHEMA %q`, schema))
	t.Cleanup(func() { _, _ = conn.Exec(ctx, fmt.Sprintf(`DROP SCHEMA %q CASCADE`, schema)) })
	mustExec(t, fmt.Sprintf(`SET search_path TO %q`, schema))

	// uniqueNames は users の unique 制約の名前を返す
	uniqueNames := func(t *testing.T) []string {
		t.Helper()
		rows, err := conn.Query(ctx, `SELECT conname FROM pg_constraint WHERE conrelid = 'users'::regclass AND contype = 'u' ORDER BY conname`)
		if err != nil {
			t.Fatal(err)
		}
		names, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			t.Fatal(err)
		}
		return names
	}
	expectUnique := func(t *testing.T, want ...string) {
		t.Helper()
		if got := uniqueNames(t); strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("unique 制約 = %v, want %v", got, want)
		}
	}
	dropUnique := emitSQL(tableChange{Table: "users", Kind: changeAlter, DropUniqueCols: []string{"email"}})
	reset := func(t *testing.T) { mustExec(t, `DROP TABLE IF EXISTS "users"`) }

	t.Run("生成した SQL で作ったテーブル", func(t *testing.T) {
		reset(t)
		mustExec(t, emitSQL(tableChange{Table: "users", Kind: changeCreate, NewTable: usersTable()}))
		expectUnique(t, "uq_users_email")
		mustExec(t, dropUnique)
		expectUnique(t)
		mustExec(t, emitSQL(tableChange{Table: "users", Kind: changeAlter, AddUniqueCols: []string{"email"}}))
		expectUnique(t, "uq_users_email")
		mustExec(t, dropUnique)
		expectUnique(t)
	})

	t.Run("ADD COLUMN で足した unique の列", func(t *testing.T) {
		reset(t)
		mustExec(t, `CREATE TABLE "users" ("id" text PRIMARY KEY)`)
		mustExec(t, emitSQL(tableChange{Table: "users", Kind: changeAlter, AddColumns: []columnSchema{
			{Name: "email", SQLType: "text", Unique: true},
		}}))
		expectUnique(t, "uq_users_email")
		mustExec(t, dropUnique)
		expectUnique(t)
	})

	t.Run("名前を明示する前の CREATE TABLE で作ったテーブル", func(t *testing.T) {
		reset(t)
		mustExec(t, `CREATE TABLE "users" ("id" text PRIMARY KEY, "email" text NOT NULL UNIQUE)`)
		expectUnique(t, "users_email_key")
		mustExec(t, dropUnique)
		expectUnique(t)
	})

	t.Run("手で別の名前を付けた制約", func(t *testing.T) {
		reset(t)
		mustExec(t, `CREATE TABLE "users" ("id" text PRIMARY KEY, "email" text NOT NULL CONSTRAINT "my_email_unique" UNIQUE)`)
		mustExec(t, dropUnique)
		expectUnique(t)
	})

	t.Run("外す制約が無ければ失敗する", func(t *testing.T) {
		reset(t)
		mustExec(t, `CREATE TABLE "users" ("id" text PRIMARY KEY, "email" text NOT NULL)`)
		if err := exec(t, dropUnique); err == nil {
			t.Fatal("unique が無いのに成功した（snapshot と DB の食い違いに気づけない）")
		}
	})
}
