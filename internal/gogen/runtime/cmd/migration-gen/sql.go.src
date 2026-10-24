package main

import (
	"fmt"
	"sort"
	"strings"
)

// emitSQL は 1 テーブルの変更を Postgres DDL に直す。CASCADE / DEFAULT は使わない方針。
// 危険な操作 (DROP TABLE / DROP COLUMN / NOT NULL 追加) は -- WARNING コメントを添える。
func emitSQL(c tableChange) string {
	var b strings.Builder

	switch c.Kind {
	case changeCreate:
		emitCreate(&b, c.NewTable)
	case changeAlter:
		emitAlter(&b, c)
	case changeDrop:
		emitDrop(&b, c.Table)
	}
	return b.String()
}

func emitCreate(b *strings.Builder, t *tableSchema) {
	fmt.Fprintf(b, "-- create table %s\n", t.Name)
	fmt.Fprintf(b, "CREATE TABLE %s (\n", quoteIdent(t.Name))

	// カラム定義は名前順で安定化、ただし PK は先頭に出すと読みやすいので
	// PK → その他の順にする。
	cols := make([]columnSchema, 0, len(t.Columns))
	for _, c := range t.Columns {
		cols = append(cols, c)
	}
	sort.Slice(cols, func(i, j int) bool {
		if cols[i].PrimaryKey != cols[j].PrimaryKey {
			return cols[i].PrimaryKey
		}
		return cols[i].Name < cols[j].Name
	})

	parts := make([]string, 0, len(cols))
	for _, c := range cols {
		parts = append(parts, "  "+columnDefSQL(c))
	}
	b.WriteString(strings.Join(parts, ",\n"))
	b.WriteString("\n);\n")

	// インデックス (単一カラム unique 以外)
	idxNames := make([]string, 0, len(t.Indexes))
	for n := range t.Indexes {
		idxNames = append(idxNames, n)
	}
	sort.Strings(idxNames)
	for _, n := range idxNames {
		emitCreateIndex(b, t.Name, t.Indexes[n])
	}
}

// columnDefSQL は CREATE TABLE 内の 1 カラム定義。DEFAULT は使わない方針なので生成しない。
func columnDefSQL(c columnSchema) string {
	parts := []string{quoteIdent(c.Name), c.SQLType}
	if c.PrimaryKey {
		parts = append(parts, "PRIMARY KEY")
	}
	if c.NotNull && !c.PrimaryKey { // PRIMARY KEY は暗黙で NOT NULL
		parts = append(parts, "NOT NULL")
	}
	if c.Unique && !c.PrimaryKey {
		parts = append(parts, "UNIQUE")
	}
	return strings.Join(parts, " ")
}

func emitAlter(b *strings.Builder, c tableChange) {
	fmt.Fprintf(b, "-- alter table %s\n", c.Table)
	tbl := quoteIdent(c.Table)

	// 順序: ADD COLUMN → ALTER COLUMN → ADD UNIQUE → DROP INDEX → ADD INDEX → DROP UNIQUE → DROP COLUMN
	// 危険操作 (DROP) を最後に置き、警告コメントを近接で確認できるようにする。
	for _, col := range c.AddColumns {
		if col.NotNull && !col.PrimaryKey {
			b.WriteString("-- WARNING: 既存行があれば NOT NULL カラムの追加は失敗します。先に値を埋めてから ALTER で NOT NULL を付けるか、本ファイルを 2 段階に分けてください。\n")
		}
		fmt.Fprintf(b, "ALTER TABLE %s ADD COLUMN %s;\n", tbl, columnDefSQL(col))
	}

	for _, ac := range c.AlterColumns {
		emitColumnAlter(b, c.Table, ac)
	}

	for _, name := range c.AddUniqueCols {
		fmt.Fprintf(b, "ALTER TABLE %s ADD CONSTRAINT %s UNIQUE (%s);\n",
			tbl, quoteIdent(uniqueConstraintName(c.Table, name)), quoteIdent(name))
	}

	for _, name := range c.DropIndexes {
		fmt.Fprintf(b, "DROP INDEX %s;\n", quoteIdent(name))
	}
	for _, idx := range c.AddIndexes {
		emitCreateIndex(b, c.Table, idx)
	}

	for _, name := range c.DropUniqueCols {
		fmt.Fprintf(b, "ALTER TABLE %s DROP CONSTRAINT %s;\n",
			tbl, quoteIdent(uniqueConstraintName(c.Table, name)))
	}

	for _, name := range c.DropColumns {
		b.WriteString("-- WARNING: DROP COLUMN は破壊的操作です。CASCADE は使わない方針なので、依存があると失敗します。\n")
		fmt.Fprintf(b, "ALTER TABLE %s DROP COLUMN %s;\n", tbl, quoteIdent(name))
	}
}

func emitColumnAlter(b *strings.Builder, table string, ac columnAlter) {
	tbl := quoteIdent(table)
	col := quoteIdent(ac.Name)

	if ac.From.SQLType != ac.To.SQLType {
		b.WriteString("-- WARNING: 型変更は USING 句が必要なケースがあります。失敗する場合は本 ALTER をコメントアウトして手動移行してください。\n")
		fmt.Fprintf(b, "ALTER TABLE %s ALTER COLUMN %s TYPE %s;\n", tbl, col, ac.To.SQLType)
	}
	if ac.From.NotNull != ac.To.NotNull {
		if ac.To.NotNull {
			b.WriteString("-- WARNING: 既存行に NULL があれば失敗します。先に値を埋めてください。\n")
			fmt.Fprintf(b, "ALTER TABLE %s ALTER COLUMN %s SET NOT NULL;\n", tbl, col)
		} else {
			fmt.Fprintf(b, "ALTER TABLE %s ALTER COLUMN %s DROP NOT NULL;\n", tbl, col)
		}
	}
	if ac.From.PrimaryKey != ac.To.PrimaryKey {
		b.WriteString("-- WARNING: PRIMARY KEY 変更は手動対応が必要です。本ファイルを参考に手書きで書き直してください。\n")
	}
}

func emitDrop(b *strings.Builder, table string) {
	fmt.Fprintf(b, "-- drop table %s\n", table)
	b.WriteString("-- WARNING: DROP TABLE は破壊的操作です。CASCADE は使わない方針なので、他テーブルから FK 参照があると失敗します。\n")
	fmt.Fprintf(b, "DROP TABLE %s;\n", quoteIdent(table))
}

func emitCreateIndex(b *strings.Builder, table string, idx indexSchema) {
	cols := make([]string, len(idx.Columns))
	for i, c := range idx.Columns {
		cols[i] = quoteIdent(c)
	}
	if idx.Unique {
		fmt.Fprintf(b, "CREATE UNIQUE INDEX %s ON %s (%s);\n",
			quoteIdent(idx.Name), quoteIdent(table), strings.Join(cols, ", "))
	} else {
		fmt.Fprintf(b, "CREATE INDEX %s ON %s (%s);\n",
			quoteIdent(idx.Name), quoteIdent(table), strings.Join(cols, ", "))
	}
}

func uniqueConstraintName(table, column string) string {
	return fmt.Sprintf("uq_%s_%s", table, column)
}

// quoteIdent は最小限の identifier クオート。Postgres は二重引用符で囲む。
// 入力に二重引用符は出ない前提 (gorm の column 名 / table 名)。
func quoteIdent(s string) string {
	return "\"" + s + "\""
}

// autoName は変更内容から migration ファイル名のサフィックスを推測する。
// 例: create_users / alter_users / drop_users
func autoName(c tableChange) string {
	switch c.Kind {
	case changeCreate:
		return "create_" + c.Table
	case changeDrop:
		return "drop_" + c.Table
	default:
		return "alter_" + c.Table
	}
}
