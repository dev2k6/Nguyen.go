package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type QueryBuilder struct {
	db         *DB
	table      string
	selectCols []string
	wheres     []whereClause
	orderBy    string
	limit      int
	offset     int
	args       []any
	argIndex   int
}

type whereClause struct {
	condition string
	args      []any
}

func (db *DB) Table(name string) *QueryBuilder {
	return &QueryBuilder{
		db:       db,
		table:    name,
		argIndex: 0,
	}
}

func (q *QueryBuilder) Select(cols ...string) *QueryBuilder {
	q.selectCols = cols
	return q
}

func (q *QueryBuilder) Where(condition string, args ...any) *QueryBuilder {
	q.wheres = append(q.wheres, whereClause{condition: condition, args: args})
	return q
}

func (q *QueryBuilder) OrderBy(clause string) *QueryBuilder {
	q.orderBy = clause
	return q
}

func (q *QueryBuilder) Limit(n int) *QueryBuilder {
	q.limit = n
	return q
}

func (q *QueryBuilder) Offset(n int) *QueryBuilder {
	q.offset = n
	return q
}

func (q *QueryBuilder) buildSelect() (string, []any) {
	cols := "*"
	if len(q.selectCols) > 0 {
		cols = strings.Join(q.selectCols, ", ")
	}

	sql := fmt.Sprintf("SELECT %s FROM %s", cols, q.table)
	var args []any
	argIdx := 1

	if len(q.wheres) > 0 {
		var conditions []string
		for _, w := range q.wheres {
			cond := w.condition
			for _, a := range w.args {
				cond = strings.Replace(cond, "?", fmt.Sprintf("$%d", argIdx), 1)
				args = append(args, a)
				argIdx++
			}
			conditions = append(conditions, cond)
		}
		sql += " WHERE " + strings.Join(conditions, " AND ")
	}

	if q.orderBy != "" {
		sql += " ORDER BY " + q.orderBy
	}
	if q.limit > 0 {
		sql += fmt.Sprintf(" LIMIT $%d", argIdx)
		args = append(args, q.limit)
		argIdx++
	}
	if q.offset > 0 {
		sql += fmt.Sprintf(" OFFSET $%d", argIdx)
		args = append(args, q.offset)
		argIdx++
	}

	return sql, args
}

func (q *QueryBuilder) Get(ctx context.Context) (*sql.Rows, error) {
	query, args := q.buildSelect()
	return q.db.Query(ctx, query, args...)
}

func (q *QueryBuilder) First(ctx context.Context) *sql.Row {
	q.limit = 1
	query, args := q.buildSelect()
	return q.db.QueryRow(ctx, query, args...)
}

func (q *QueryBuilder) Count(ctx context.Context) (int64, error) {
	sql := fmt.Sprintf("SELECT COUNT(*) FROM %s", q.table)
	var args []any
	argIdx := 1

	if len(q.wheres) > 0 {
		var conditions []string
		for _, w := range q.wheres {
			cond := w.condition
			for _, a := range w.args {
				cond = strings.Replace(cond, "?", fmt.Sprintf("$%d", argIdx), 1)
				args = append(args, a)
				argIdx++
			}
			conditions = append(conditions, cond)
		}
		sql += " WHERE " + strings.Join(conditions, " AND ")
	}

	var count int64
	err := q.db.QueryRow(ctx, sql, args...).Scan(&count)
	return count, err
}

func (q *QueryBuilder) Insert(ctx context.Context, data map[string]any) (sql.Result, error) {
	var cols []string
	var placeholders []string
	var args []any
	idx := 1

	for col, val := range data {
		cols = append(cols, col)
		placeholders = append(placeholders, fmt.Sprintf("$%d", idx))
		args = append(args, val)
		idx++
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		q.table,
		strings.Join(cols, ", "),
		strings.Join(placeholders, ", "),
	)

	return q.db.Exec(ctx, query, args...)
}

func (q *QueryBuilder) InsertReturning(ctx context.Context, data map[string]any, returning string) *sql.Row {
	var cols []string
	var placeholders []string
	var args []any
	idx := 1

	for col, val := range data {
		cols = append(cols, col)
		placeholders = append(placeholders, fmt.Sprintf("$%d", idx))
		args = append(args, val)
		idx++
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) RETURNING %s",
		q.table,
		strings.Join(cols, ", "),
		strings.Join(placeholders, ", "),
		returning,
	)

	return q.db.QueryRow(ctx, query, args...)
}

func (q *QueryBuilder) Update(ctx context.Context, data map[string]any) (sql.Result, error) {
	var sets []string
	var args []any
	idx := 1

	for col, val := range data {
		sets = append(sets, fmt.Sprintf("%s = $%d", col, idx))
		args = append(args, val)
		idx++
	}

	query := fmt.Sprintf("UPDATE %s SET %s", q.table, strings.Join(sets, ", "))

	if len(q.wheres) > 0 {
		var conditions []string
		for _, w := range q.wheres {
			cond := w.condition
			for _, a := range w.args {
				cond = strings.Replace(cond, "?", fmt.Sprintf("$%d", idx), 1)
				args = append(args, a)
				idx++
			}
			conditions = append(conditions, cond)
		}
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	return q.db.Exec(ctx, query, args...)
}

func (q *QueryBuilder) Delete(ctx context.Context) (sql.Result, error) {
	query := fmt.Sprintf("DELETE FROM %s", q.table)
	var args []any
	idx := 1

	if len(q.wheres) > 0 {
		var conditions []string
		for _, w := range q.wheres {
			cond := w.condition
			for _, a := range w.args {
				cond = strings.Replace(cond, "?", fmt.Sprintf("$%d", idx), 1)
				args = append(args, a)
				idx++
			}
			conditions = append(conditions, cond)
		}
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	return q.db.Exec(ctx, query, args...)
}
