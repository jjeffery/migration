package migration

import (
	"reflect"
	"testing"

	"github.com/davecgh/go-spew/spew"
)

func TestNewDDLActions(t *testing.T) {
	tests := []struct {
		sql     string
		actions ddlActions
	}{
		{
			sql: "create table t1",
			actions: ddlActions{
				&ddlAction{
					verb:       ddlVerbCreate,
					objectType: dbObjectTypeTable,
					name:       "t1",
				},
			},
		},
		{
			sql: "drop table if exists t1;create table t1",
			actions: ddlActions{
				&ddlAction{
					verb:        ddlVerbCreate,
					objectType:  dbObjectTypeTable,
					name:        "t1",
					dropBefore:  true,
					checkExists: true,
				},
			},
		},
		{
			sql:     "something not supported",
			actions: nil,
		},
		{
			sql: "create table s.t1; create index on s.t1;",
			actions: ddlActions{
				&ddlAction{
					verb:       ddlVerbCreate,
					objectType: dbObjectTypeTable,
					name:       "t1",
					schema:     "s",
				},
			},
		},
		{
			sql: "create table t1; alter table t1 set whatever;",
			actions: ddlActions{
				&ddlAction{
					verb:       ddlVerbCreate,
					objectType: dbObjectTypeTable,
					name:       "t1",
				},
			},
		},
		{
			sql: "create table t1; create table t2; create index on t1; alter table t1; alter table t2; create index on t2",
			actions: ddlActions{
				&ddlAction{
					verb:       ddlVerbCreate,
					objectType: dbObjectTypeTable,
					name:       "t1",
				},
				&ddlAction{
					verb:       ddlVerbCreate,
					objectType: dbObjectTypeTable,
					name:       "t2",
				},
			},
		},
	}

	for tn, tt := range tests {
		if got, want := newDDLActions(tt.sql), tt.actions; !actionsEqual(got, want) {
			t.Errorf("%d: got=%v\nwant=%v", tn, spew.Sdump(got), spew.Sdump(want))
		}
	}
}

func actionsEqual(acts1, acts2 ddlActions) bool {
	if len(acts1) != len(acts2) {
		return false
	}
	for i, act1 := range acts1 {
		if !reflect.DeepEqual(*act1, *acts2[i]) {
			return false
		}
	}
	return true
}

func TestNewStatements(t *testing.T) {
	tests := []struct {
		sql   string
		stmts []statement
	}{
		{
			sql: `DROP TABLE IF EXISTS t1; CREATE TABLE t1(id INT PRIMARY KEY, name TEXT);`,
			stmts: []statement{
				statement{"drop", "table", "if", "exists", "t1", ";"},
				statement{"create", "table", "t1", "(", "id", "INT", "PRIMARY", "KEY", ",", "name", "TEXT", ")", ";"},
			},
		},
		{
			sql: "-- comment\nDROP TABLE IF EXISTS t1;",
			stmts: []statement{
				statement{"drop", "table", "if", "exists", "t1", ";"},
			},
		},
		{
			sql: "missing trailing semicolon",
			stmts: []statement{
				statement{"missing", "trailing", "semicolon"},
			},
		},
	}

	for tn, tt := range tests {
		stmts := newStatements(tt.sql)
		if got, want := stmts, tt.stmts; !reflect.DeepEqual(got, want) {
			t.Errorf("%d: got=%v\nwant=%v", tn, got, want)
		}
	}
}

func TestStatementGet(t *testing.T) {
	tests := []struct {
		stmt  statement
		index int
		want  string
	}{
		{
			stmt:  statement{"0", "1"},
			index: 0,
			want:  "0",
		},
		{
			stmt:  statement{"0", "1"},
			index: 1,
			want:  "1",
		},
		{
			stmt:  statement{"0", "1"},
			index: 2,
			want:  "",
		},
		{
			stmt:  nil,
			index: 0,
			want:  "",
		},
	}
	for tn, tt := range tests {
		if got, want := tt.stmt.get(tt.index), tt.want; got != want {
			t.Errorf("%d: got=%v, want=%v", tn, got, want)
		}
	}
}

func TestStatementMatch(t *testing.T) {
	tests := []struct {
		stmt    statement
		lexemes []string
		want    bool
	}{
		{
			stmt:    statement{"0", "1"},
			lexemes: []string{"0", "1"},
			want:    true,
		},
		{
			stmt:    statement{"0", "1"},
			lexemes: []string{"0"},
			want:    true,
		},
		{
			stmt:    statement{"0", "1"},
			lexemes: []string{"0", "1", "2"},
			want:    false,
		},
		{
			stmt:    nil,
			lexemes: []string{"0"},
			want:    false,
		},
	}
	for tn, tt := range tests {
		if got, want := tt.stmt.match(tt.lexemes...), tt.want; got != want {
			t.Errorf("%d: got=%v, want=%v", tn, got, want)
		}
	}
}

func TestStatementRemove(t *testing.T) {
	tests := []struct {
		stmt    statement
		lexemes []string
		want    statement
	}{
		{
			stmt:    statement{"0", "1"},
			lexemes: []string{"0", "1"},
			want:    statement{},
		},
		{
			stmt:    statement{"0", "1"},
			lexemes: []string{"0"},
			want:    statement{"1"},
		},
		{
			stmt:    statement{"0", "1"},
			lexemes: []string{"0", "1", "2"},
			want:    statement{"0", "1"},
		},
		{
			stmt:    nil,
			lexemes: []string{"0"},
			want:    nil,
		},
	}
	for tn, tt := range tests {
		if got, want := tt.stmt.remove(tt.lexemes...), tt.want; !reflect.DeepEqual(got, want) {
			t.Errorf("%d: got=%v, want=%v", tn, got, want)
		}
	}
}
