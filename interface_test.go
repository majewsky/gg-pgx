// SPDX-FileCopyrightText: 2026 Stefan Majewsky <majewsky@gmx.net>
// SPDX-License-Identifier: Apache-2.0

package gg_pgx_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"go.xyrillian.de/gg/assert"
	"go.xyrillian.de/gg/errext"
	"go.xyrillian.de/gg/gsql"
	"go.xyrillian.de/gg/pgruntime"

	gg_pgx "go.xyrillian.de/gg-pgx"
)

// NOTE: There is not a lot of test coverage for the methods in `interface.go`, which is intentional:
// These are just boilerplate implementations where, if it compiles at all, it is all but guaranteed to work as intended.

var defaultBehavior pgruntime.ConnectionBehavior

func TestMain(m *testing.M) {
	pgruntime.WithTestDB(m, m.Run)
}

func TestConn(t *testing.T) {
	conn, _ := gg_pgx.SingleConnector().ConnectForTest(t, defaultBehavior)
	testConnectionHandle(t, conn)
}

func TestPool(t *testing.T) {
	pool, _ := gg_pgx.PoolConnector().ConnectForTest(t, defaultBehavior)
	testConnectionHandle(t, pool)
}

func TestPoolConn(t *testing.T) {
	pool, _ := gg_pgx.PoolConnector().ConnectForTest(t, defaultBehavior)
	t.Cleanup(pool.Close)

	conn, err := pool.Acquire(t.Context())
	if assert.ErrEqual(t, err, nil) {
		testConnectionHandle(t, conn)
	}
}

func testConnectionHandle[H gsql.ConnectionHandle](t *testing.T, conn H) {
	ctx := t.Context()
	t.Cleanup(func() {
		assert.ErrEqual(t, conn.GSQLClose(ctx), nil)
	})

	// test GSQLPrepare() + Exec()
	_, err := execQuery(ctx, conn, `CREATE TABLE people (id BIGSERIAL PRIMARY KEY, name TEXT NOT NULL)`, nil)
	if !assert.ErrEqual(t, err, nil) {
		t.FailNow()
	}
	for _, name := range []string{"Alice", "Bob", "Carol"} {
		_, err = execQuery(ctx, conn, `INSERT INTO people (name) VALUES ($1)`, []any{name})
		if !assert.ErrEqual(t, err, nil) {
			t.FailNow()
		}
	}

	// test GSQLPrepare() + QueryRow()
	var id int64
	err = queryRow(ctx, conn, `SELECT id FROM people WHERE name = $1`, []any{"Bob"}, []any{&id})
	if !assert.ErrEqual(t, err, nil) {
		t.FailNow()
	}
	assert.Equal(t, id, 2)

	// test GSQLQuery()
	type person struct {
		ID   int64
		Name string
	}
	var people []person
	err = foreachRow(t, ctx, conn, `SELECT id, name FROM people ORDER BY id`, nil, []string{"id", "name"}, func(scan func(slots ...any) error) error {
		var p person
		err := scan(&p.ID, &p.Name)
		if err != nil {
			return err
		}
		people = append(people, p)
		return nil
	})
	if !assert.ErrEqual(t, err, nil) {
		t.FailNow()
	}
	assert.Equal(t, people, []person{
		{1, "Alice"},
		{2, "Bob"},
		{3, "Carol"},
	})

	// test GSQLTransact() + success
	err = conn.GSQLTransact(ctx, func(tx gsql.Handle) error {
		_, err := execQuery(ctx, tx, `UPDATE people SET name = $1 WHERE id = $2`, []any{"Carolin", 3})
		return err
	})
	if !assert.ErrEqual(t, err, nil) {
		t.FailNow()
	}

	// check that the transaction was committed
	var name string
	err = queryRow(ctx, conn, `SELECT name FROM people WHERE id = $1`, []any{3}, []any{&name})
	if !assert.ErrEqual(t, err, nil) {
		t.FailNow()
	}
	assert.Equal(t, name, "Carolin")

	// test GSQLTransact() + error
	err = conn.GSQLTransact(ctx, func(tx gsql.Handle) error {
		_, err := execQuery(ctx, tx, `UPDATE people SET name = $1 WHERE id = $2`, []any{"Allen", 1})
		if err != nil {
			return err
		}

		// check that the change is visible within the transaction
		var name string
		err = queryRow(ctx, tx, `SELECT name FROM people WHERE id = $1`, []any{1}, []any{&name})
		if !assert.ErrEqual(t, err, nil) {
			t.FailNow()
		}
		assert.Equal(t, name, "Allen")

		return errors.New("this was a bad idea")
	})
	if !assert.ErrEqual(t, err, "this was a bad idea") {
		t.FailNow()
	}

	// check that the transaction was *not* committed
	err = queryRow(ctx, conn, `SELECT name FROM people WHERE id = $1`, []any{1}, []any{&name})
	if !assert.ErrEqual(t, err, nil) {
		t.FailNow()
	}
	assert.Equal(t, name, "Alice")
}

// Convenience function for executing a one-off SQL query returning no rows.
func execQuery(ctx context.Context, db gsql.Handle, query string, args []any) (sql.Result, error) {
	stmt, err := db.GSQLPrepare(ctx, query, false)
	if err != nil {
		return nil, err
	}
	result, err := stmt.Exec(ctx, args)
	return result, errext.WithCleanup(err, "stmt.Close", stmt.Close())
}

// Convenience function for executing a one-off SQL query returning one row.
func queryRow(ctx context.Context, db gsql.Handle, query string, args, slots []any) error {
	stmt, err := db.GSQLPrepare(ctx, query, true) // repeated=true does not technically make sense, but provides code coverage
	if err != nil {
		return err
	}
	err = stmt.QueryRow(ctx, args, slots)
	return errext.WithCleanup(err, "stmt.Close", stmt.Close())
}

// Convenience function for executing a one-off SQL query returning several rows.
func foreachRow(t *testing.T, ctx context.Context, db gsql.Handle, query string, args []any, expectedColumnNames []string, action func(scan func(slots ...any) error) error) error {
	t.Helper()
	rows, err := db.GSQLQuery(ctx, query, args)
	if err != nil {
		return err
	}
	columns, err := rows.Columns()
	if assert.ErrEqual(t, err, nil) {
		assert.Equal(t, columns, expectedColumnNames)
	}
	for rows.Next() {
		err := action(rows.Scan)
		if err != nil {
			return errext.WithCleanup(err, "rows.Close", rows.Close())
		}
	}
	return rows.Close()
}
