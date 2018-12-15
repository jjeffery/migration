package migration

import "fmt"

// A Definition is used to define a database version, the actions
// required to migrate up from the previous version, and the
// actions required to migrate down to the previous version.
type Definition struct {
	id          int
	description string
	upSQL       string
	upDB        DBFunc
	upTx        TxFunc
	downSQL     string
	downDB      DBFunc
	downTx      TxFunc
}

func newDefinition(schema *Schema, id int) *Definition {
	return &Definition{
		id: id,
	}
}

// Description sets the description for the database version.
//
// The description can usually be inferred from the SQL
// provided to the Up() method. For example, if the up
// SQL contains a single CREATE TABLE statement, then the
// description will be set to "table <table-name>".
//
// Use this method to override the default description, or to
// set a description when none can be automatically determined.
func (d *Definition) Description(s string) *Definition {
	d.description = s
	return d
}

// Up defines the SQL to migrate up to the version.
//
// If the sql contains a single statement and it is
// one of the following SQL statements, then the
// SQL for migrating to the previous version is automatically
// generated.
//  command            down migration
//  -------            --------------
//  CREATE TABLE       drop table
//  CREATE INDEX       drop index
//  CREATE DOMAIN      drop domain
//  CREATE VIEW        revert to previous view, or drop view if none
//  CREATE TRIGGER     revert to previous trigger, or drop trigger if none
//  CREATE PROCEDURE   revert to previous procedure, or drop procedure if none
func (d *Definition) Up(sql string) *Definition {
	d.upSQL = sql
	return d
}

// Down defines the SQL to migrate down to the previous version.
//
// The Down() method is often optional. See the Up() method for details of when
// the SQL for the down migration is automatically generated.
func (d *Definition) Down(sql string) *Definition {
	d.downSQL = sql
	return d
}

// UpTx defines a function that implements the migration up from the
// previous version.
//
// The up migration is performed inside a transaction,
// so the if the migration fails for any reason, the database will
// rollback to its state at the start version.
func (d *Definition) UpTx(up TxFunc) *Definition {
	d.upTx = up
	return d
}

// DownTx defines a function that implements the migration down to
// the previous version.
//
// The down migration is performed inside a transaction,
// so the if the migration fails for any reason, the database will
// rollback to its state at the start version.
func (d *Definition) DownTx(down TxFunc) *Definition {
	d.downTx = down
	return d
}

// UpDB defines a function that implements the migration up from the
// previous version.
//
// The up migration is performed outside of a transaction, so
// if the migration fails for any reason, the database will
// require manual repair before any more migrations can proceed.
// If possible, use UpTx to perform migrations within a
// database transaction.
func (d *Definition) UpDB(up DBFunc) *Definition {
	d.upDB = up
	return d
}

// DownDB defines a function that implements the migration down to
// the previous version.
//
// The down migration is performed outside of a transaction, so
// if the migration fails for any reason, the database will
// require manual repair before any more migrations can proceed.
// If possible, use DownTx to perform migrations within a
// database transaction.
func (d *Definition) DownDB(down DBFunc) *Definition {
	d.downDB = down
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

	{
		var upMethods []string
		if d.upSQL != "" {
			upMethods = append(upMethods, "Up")
		}
		if d.upDB != nil {
			upMethods = append(upMethods, "UpDB")
		}
		if d.upTx != nil {
			upMethods = append(upMethods, "UpTx")
		}
		if len(upMethods) > 1 {
			addError(fmt.Sprintf("call only one method of %v", upMethods))
		}
		if len(upMethods) == 0 {
			addError("must call one of [Up UpDB UpTx]")
		}
	}

	{
		downMethods := d.downMethods()
		if len(downMethods) > 1 {
			addError(fmt.Sprintf("call only one method of %v", downMethods))
		}
	}

	return errs
}

func (d *Definition) downMethods() []string {
	var downMethods []string
	if d.downSQL != "" {
		downMethods = append(downMethods, "Down")
	}
	if d.downDB != nil {
		downMethods = append(downMethods, "DownDB")
	}
	if d.downTx != nil {
		downMethods = append(downMethods, "DownTx")
	}
	return downMethods
}
