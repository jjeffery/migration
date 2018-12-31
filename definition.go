package migration

import "fmt"

// A Definition is used to define a database version, the actions
// required to migrate up from the previous version, and the
// actions required to migrate down to the previous version.
type Definition struct {
	id      VersionID
	upSQL   string
	upDB    DBFunc
	upTx    TxFunc
	downSQL string
	downDB  DBFunc
	downTx  TxFunc
}

func newDefinition(id VersionID) *Definition {
	return &Definition{
		id: id,
	}
}

// Up defines the SQL to migrate up to the version.
func (d *Definition) Up(sql string) *Definition {
	d.upSQL = sql
	return d
}

// Down defines the SQL to migrate down to the previous version.
//
// The Down() method is optional if the corresponding Up() method
// contains a single CREATE VIEW statement. The automatically-generated
// down migration restores the previous version of the view, or if
// there is no previous version then the view is dropped.
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
			addError(fmt.Sprintf("call only one of %v", upMethods))
		}
		if len(upMethods) == 0 {
			addError("call one of [Up UpDB UpTx]")
		}
	}

	{
		downMethods := d.downMethods()
		if len(downMethods) > 1 {
			addError(fmt.Sprintf("call only one of %v", downMethods))
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
