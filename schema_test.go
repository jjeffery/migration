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
				"1: call one of [Up UpDB UpTx]",
			},
		},
		{
			fn: func(s *Schema) {
				s.Define(1).
					Down("do something").
					Up("do something").
					UpDB(func(ctx context.Context, db *sql.DB) error { return nil })
			},
			errs: []string{
				"1: call only one of [Up UpDB]",
			},
		},
		{
			fn: func(s *Schema) {
				s.Define(1).
					Down("do something").
					UpTx(func(ctx context.Context, db *sql.Tx) error { return nil }).
					UpDB(func(ctx context.Context, db *sql.DB) error { return nil })
			},
			errs: []string{
				"1: call only one of [UpDB UpTx]",
			},
		},
		{
			fn: func(s *Schema) {
				s.Define(1).
					Up("do something").
					Down("do something").
					DownDB(func(ctx context.Context, db *sql.DB) error { return nil })
			},
			errs: []string{
				"1: call only one of [Down DownDB]",
			},
		},
		{
			fn: func(s *Schema) {
				s.Define(1).
					Up("do something").
					DownTx(func(ctx context.Context, db *sql.Tx) error { return nil }).
					DownDB(func(ctx context.Context, db *sql.DB) error { return nil })
			},
			errs: []string{
				"1: call only one of [DownDB DownTx]",
			},
		},
		{
			fn: func(s *Schema) {
				s.Define(1).Up("create table t1(id int);")
				s.Define(2).Up("unknown DDL command")
			},
			errs: []string{
				"2: call one of [Down DownDB DownTx]",
			},
		},
		{
			fn: func(s *Schema) {
				s.Define(1).Up("create table t1(id int);" +
					"create view v1 as select * from t1;").
					Down("drop view v1; drop table t1;")

			},
			errs: []string{
				"1: create view v1 in its own migration",
			},
		},
		{
			fn: func(s *Schema) {
				s.Define(1).Up("create table t1(id int);drop table t2;")

			},
			errs: []string{
				"1: drop table t2 needs a manual down migration",
				"1: call one of [Down DownDB DownTx]",
			},
		},
		{
			fn: func(s *Schema) {
				s.Define(1).Up("create table t1(id int);")
				s.Define(3).Up("alter table t1;")

			},
			errs: []string{
				"3: alter table t1 needs a manual down migration",
				"3: call one of [Down DownDB DownTx]",
			},
		},
		{
			fn: func(s *Schema) {
				s.Define(2).Up("create index i1 on t1(id);")
				s.Define(3).Up("create index on t2(id);")
			},
			errs: []string{
				"2: create index i1 on t1 needs a manual down migration",
				"3: create index on t2 needs a manual down migration",
			},
		},
		{
			fn: func(s *Schema) {
				s.Define(2).Up("create trigger tr1 on t1;")
			},
			errs: []string{
				"2: create trigger tr1 on t1 needs a manual down migration",
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

func TestSchemaDerivedDownSQL(t *testing.T) {
	tests := []struct {
		fn       func(s *Schema)
		downSQLs []string
	}{
		{
			fn: func(s *Schema) {
				s.Define(1).Up("create table t1;")
			},
			downSQLs: []string{
				"drop table t1;\n",
			},
		},
		{
			fn: func(s *Schema) {
				s.Define(1).Up("create table t1;create index on t1;alter table t1;")
			},
			downSQLs: []string{
				"drop table t1;\n",
			},
		},
		{
			fn: func(s *Schema) {
				s.Define(1).Up("create view v1 as select * from t1;")
				s.Define(2).Up("drop view v1; create view v1 as select * from t2;")

			},
			downSQLs: []string{
				"drop view v1;\n",
				"drop view v1;\ncreate view v1 as select * from t1;",
			},
		},
		{
			fn: func(s *Schema) {
				s.Define(1).Up("create domain d1; create table t1;")
			},
			downSQLs: []string{
				"drop table t1;\ndrop domain d1;\n",
			},
		},

		/*
			{
				fn: func(s *Schema) {

				},
				downSQLs: []string{},
			},
		*/
	}
	for tn, tt := range tests {
		var s Schema
		tt.fn(&s)
		if err := s.Err(); err != nil {
			t.Errorf("%d: %v", tn, err)
			continue
		}
		var downSQLs []string
		for _, plan := range s.plans {
			downSQLs = append(downSQLs, plan.downSQL)
		}

		if got, want := downSQLs, tt.downSQLs; !reflect.DeepEqual(got, want) {
			t.Errorf("%d:\ngot=%v\bwant=%v", tn, got, want)
		}
	}
}
