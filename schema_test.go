package migration

import (
	"reflect"
	"strings"
	"testing"
)

func TestSchemaDefine(t *testing.T) {
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

func TestSchemaErr(t *testing.T) {
	var s Schema

	s.Define(1)
	s.Define(1)

	e1 := s.Err()
	e2 := s.Err()

	if !reflect.DeepEqual(e1, e2) {
		t.Errorf("got=%v\n\nwant=%v\n", e1, e2)
	}
}
