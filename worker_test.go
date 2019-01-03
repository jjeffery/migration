package migration

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

func TestWorker(t *testing.T) {
	tests := []struct {
		driver string
		dsn    string
	}{
		{
			driver: "sqlite3",
			dsn:    ":memory:",
		},
		{
			driver: "postgres",
			dsn:    "postgres://migration_test:migration_test@localhost/migration_test?sslmode=disable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.driver, func(t *testing.T) {
			ctx := context.Background()
			db, err := sql.Open(tt.driver, tt.dsn)
			wantNoError(t, err)
			defer db.Close()

			worker, err := NewWorker(db, newTestSchema())
			wantNoError(t, err)

			err = worker.Up(ctx)
			wantNoError(t, err)

			// remove everything at the end of the test
			defer func() {
				wantNoError(t, worker.Goto(ctx, 0))
			}()

			err = worker.Down(ctx)
			wantNoError(t, err)

			err = worker.Goto(ctx, 3)
			wantError(t, err, "invalid schema version id=3")

			err = worker.Goto(ctx, 20)
			wantNoError(t, err)

			err = worker.Lock(ctx, 20)
			wantNoError(t, err)

			err = worker.Goto(ctx, 10)
			wantError(t, err, "database schema version locked id=20")

			err = worker.Goto(ctx, 0)
			wantError(t, err, "database schema version locked id=20")

			err = worker.Unlock(ctx, 20)
			wantNoError(t, err)

			err = worker.Goto(ctx, 10)
			wantNoError(t, err)

			err = worker.Goto(ctx, 0)
			wantNoError(t, err)

			ver, err := worker.Version(ctx, 20)
			wantNoError(t, err)

			if ver.AppliedAt != nil {
				t.Fatalf("got=%v, want=nil", *ver.AppliedAt)
			}

			err = worker.Up(ctx)
			wantNoError(t, err)

			ver, err = worker.Version(ctx, 20)
			wantNoError(t, err)
			if ver.AppliedAt == nil {
				t.Fatal("got=nil, want=non-nil")
			}

			_, err = worker.Version(ctx, 19)
			wantError(t, err, "invalid schema version id=19")

			vers, err := worker.Versions(ctx)
			wantNoError(t, err)
			if got, want := len(vers), 2; got != want {
				t.Fatalf("got=%v, want=%v", got, want)
			}
		})
	}
}

func wantNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func wantError(t *testing.T, err error, contains string) {
	t.Helper()
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if contains != "" {
		if !strings.Contains(err.Error(), contains) {
			t.Fatalf("want=%v, got=%v", contains, err.Error())
		}
	}
}

func newTestSchema() *Schema {
	var schema Schema

	schema.Define(10).Up(`
		create table t1(
			id int primary key,
			name varchar(30)
		);

		create index i1 on t1(name);
	`).Down(`
		drop table t1;
	`)

	schema.Define(20).Up(`
		create table t2(
			id int primary key,
			name varchar(30)
		);

		create index i2 on t2(name);
	`).Down(`
		drop table t2;
	`)

	return &schema
}
