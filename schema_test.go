package migration

import (
	"context"
	"database/sql"
	"reflect"
	"strings"
	"testing"
)

func TestSchemaErrors(t *testing.T) {
	tests := []struct {
		fn   func(s *Schema)
		errs []string
	}{
		{
			fn: func(s *Schema) {
				s.Define(1).
					Up(`create table t1(id int primary key, name text;`).
					Down(`drop table t1;`)
			},
		},
		{
			fn: func(s *Schema) {
				s.Define(1).
					Up(`create table t1(id int primary key, name text;`).
					Down(`drop table t1;`)
				s.Define(1)
			},
			errs: []string{
				"1: defined more than once",
			},
		},
		{
			fn: func(s *Schema) {
				s.Define(1).Down("do something")
			},
			errs: []string{
				"1: up migration not defined",
			},
		},
		{
			fn: func(s *Schema) {
				s.Define(1).
					Down("do something").
					Up("do something").
					UpAction(DBFunc(func(ctx context.Context, db *sql.DB) error { return nil }))
			},
			errs: []string{
				"1: up migration defined 2 times",
			},
		},
		{
			fn: func(s *Schema) {
				s.Define(1).
					Down("do something").
					UpAction(TxFunc(func(ctx context.Context, db *sql.Tx) error { return nil })).
					UpAction(DBFunc(func(ctx context.Context, db *sql.DB) error { return nil }))
			},
			errs: []string{
				"1: up migration defined 2 times",
			},
		},
		{
			fn: func(s *Schema) {
				s.Define(1).
					Up("do something").
					Down("do something").
					DownAction(DBFunc(func(ctx context.Context, db *sql.DB) error { return nil }))
			},
			errs: []string{
				"1: down migration defined 2 times",
			},
		},
		{
			fn: func(s *Schema) {
				s.Define(1).
					Up("do something").
					DownAction(TxFunc(func(ctx context.Context, db *sql.Tx) error { return nil })).
					DownAction(DBFunc(func(ctx context.Context, db *sql.DB) error { return nil }))
			},
			errs: []string{
				"1: down migration defined 2 times",
			},
		},
		{
			fn: func(s *Schema) {
				s.Define(1).Up("create table t1(id int);").Down("drop table t1;")
				s.Define(2).Up("some DDL command")
			},
			errs: []string{
				"2: down migration not defined",
			},
		},
		{
			fn: func(s *Schema) {
				s.Define(9).UpAction(Replay(8)).Down(`-- noop`)

			},
			errs: []string{
				"9: replay refers to unknown version 8",
			},
		},
		{
			fn: func(s *Schema) {
				s.Define(9).UpAction(Replay(10)).Down(`-- noop`)

			},
			errs: []string{
				"9: replay must specify an earlier version",
			},
		},
	}

	for tn, tt := range tests {
		var s Schema
		tt.fn(&s)
		errs, _ := s.Err().(Errors)
		var errTexts []string
		for _, e := range errs {
			errTexts = append(errTexts, e.Error())
		}
		if got, want := strings.Join(errTexts, "\n"), strings.Join(tt.errs, "\n"); got != want {
			t.Errorf("%d:\ngot:\n%s\n\nwant:\n%s\n\n", tn, got, want)
		}
	}
}

func TestSchemaCannotCreateNewCommand(t *testing.T) {
	var s Schema

	s.Define(1)
	s.Define(1)

	// cannot create a new worker when schema has errors
	e1 := s.Err()
	_, e2 := NewWorker(&sql.DB{}, &s)

	if !reflect.DeepEqual(e1, e2) {
		t.Errorf("got=%v\n\nwant=%v\n", e1, e2)
	}
}

func TestSchemaReplay(t *testing.T) {
	tests := []struct {
		fn   func(s *Schema) string
		want string
	}{
		{
			fn: func(s *Schema) string {
				s.Define(1).Up("create view v1;").Down("drop view v1;")
				s.Define(2).Up("drop view v1;").DownAction(Replay(1))
				s.complete()
				return s.plans[1].down.sql
			},
			want: "create view v1;",
		},
		{
			fn: func(s *Schema) string {
				s.Define(1).Up("create view v1;").Down("drop view v1;")
				s.Define(2).Up("drop view v1;").DownAction(Replay(1))
				s.Define(3).UpAction(Replay(1)).Down("drop view v1;")
				s.Define(4).Up("drop view v1;").DownAction(Replay(1))
				s.Define(5).UpAction(Replay(3)).Down("drop view v1;")
				s.complete()
				return s.plans[4].up.sql
			},
			want: "create view v1;",
		},
	}
	for tn, tt := range tests {
		var s Schema
		sql := tt.fn(&s)
		if err := s.Err(); err != nil {
			t.Errorf("%d: %v", tn, err)
			continue
		}

		if got, want := sql, tt.want; got != want {
			t.Errorf("%d:\ngot=%v\bwant=%v", tn, got, want)
		}
	}
}
