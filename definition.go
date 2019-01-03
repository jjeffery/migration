package migration

import (
	"context"
	"database/sql"
	"fmt"
)

// A Definition is used to define a database schema version, the action
// required to migrate up from the previous version, and the
// action required to migrate down to the previous version.
type Definition struct {
	id         VersionID
	upAction   Action
	upCount    int
	downAction Action
	downCount  int
}

func newDefinition(id VersionID) *Definition {
	return &Definition{
		id: id,
	}
}

// Up defines the SQL to migrate up to the version.
//
// Calling this function is identical to calling:
//  UpAction(Command(sql))
func (d *Definition) Up(sql string) *Definition {
	d.upCount++
	d.upAction = Command(sql)
	return d
}

// UpAction defines the action to perform during the migration up
// to this database schema version.
//
// The Up() method handles the most common case for up migrations.
func (d *Definition) UpAction(a Action) *Definition {
	d.upCount++
	d.upAction = a
	return d
}

// Down defines the SQL to migrate down to the previous version.
//
// Calling this function is identical to calling:
//  DownAction(Command(sql))
func (d *Definition) Down(sql string) *Definition {
	d.downCount++
	d.downAction = Command(sql)
	return d
}

// DownAction defines the action to perform during the migration down
// from this database schema version.
//
// The Down() method handles the most common case for up migrations.
func (d *Definition) DownAction(a Action) *Definition {
	d.downCount++
	d.downAction = a
	return d
}

func (d *Definition) errs() Errors {
	var errs Errors

	addError := func(s string) {
		errs = append(errs, &Error{
			Version:     d.id,
			Description: s,
		})
	}

	if d.upCount == 0 {
		addError("up migration not defined")
	}
	if d.upCount > 1 {
		addError(fmt.Sprintf("up migration defined %d times", d.upCount))
	}

	if d.downCount == 0 {
		addError("down migration not defined")
	}
	if d.downCount > 1 {
		addError(fmt.Sprintf("down migration defined %d times", d.downCount))
	}

	return errs
}

type action struct {
	sql      string
	dbFunc   func(context.Context, *sql.DB) error
	txFunc   func(context.Context, *sql.Tx) error
	replayUp *VersionID
}

// An Action defines the action performed during an up migration or
// a down migration.
type Action func(*action)

// Command returns an action that executes the SQL/DDL command.
//
// Command is by far the most common migration action. The Up()
// and Down() methods provide a quick way to define migration
// actions when they are SQL/DDL commands.
func Command(sql string) Action {
	return func(a *action) {
		a.sql = sql
	}
}

// DBFunc returns an action that executes the function f.
//
// The migration is performed outside of a transaction, so
// if the migration fails for any reason, the database will
// require manual repair before any more migrations can proceed.
// If possible, use TxFunc to perform migrations within a
// database transaction.
func DBFunc(f func(context.Context, *sql.DB) error) Action {
	return func(a *action) {
		a.dbFunc = f
	}
}

// TxFunc returns an action that executes function f.
//
// The migration is performed inside a transaction, so
// if the migration fails for any reason, the database will
// rollback to its state at the start version.
func TxFunc(f func(context.Context, *sql.Tx) error) Action {
	return func(a *action) {
		a.txFunc = f
	}
}

// Replay returns an action that replays the up migration for the
// specified database version.
//
// Replay actions are useful for restoring views, functions and
// stored procedures
func Replay(id VersionID) Action {
	return func(a *action) {
		a.replayUp = &id
	}
}
