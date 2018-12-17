package migration

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// A Command performs database migrations. It combines the
// information in the migration schema along with the database
// on which to perform migrations.
type Command struct {
	// LogFunc is a function for logging progress. If not specified then
	// no logging is performed.
	//
	// One common practice is to assign the log.Println function to LogFunc.
	LogFunc func(v ...interface{})

	schema     *Schema
	db         *sql.DB
	drv        driver
	initCalled bool
}

// NewCommand creates a command that can perform migrations for
// the specified database using the database migration schema.
func NewCommand(db *sql.DB, schema *Schema) (*Command, error) {
	if err := schema.Err(); err != nil {
		return nil, err
	}
	drv, err := findDriver(db)
	if err != nil {
		return nil, err
	}
	cmd := &Command{
		schema: schema,
		db:     db,
		drv:    drv,
	}
	return cmd, nil
}

// Up migrates the database to the latest version.
func (m *Command) Up(ctx context.Context) error {
	if err := m.init(ctx); err != nil {
		return err
	}
	for {
		more, err := m.upOne(ctx)
		if err != nil {
			return err
		}
		if !more {
			break
		}
	}
	return nil
}

// Down migrates the database down to the latest locked version.
// If there are no locked versions, all down migrations are performed.
func (m *Command) Down(ctx context.Context) error {
	if err := m.init(ctx); err != nil {
		return err
	}
	for {
		more, err := m.downOne(ctx)
		if err != nil {
			return err
		}
		if !more {
			break
		}
	}
	return nil
}

// Version returns the current version of the database schema.
func (m *Command) Version(ctx context.Context) (*Version, error) {
	if err := m.init(ctx); err != nil {
		return nil, err
	}
	var version *Version
	err := m.transact(ctx, func(tx *sql.Tx) error {
		versions, err := m.listVersions(ctx, tx)
		if err != nil {
			return err
		}
		if len(versions) > 0 {
			version = versions[len(versions)-1]
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return version, nil
}

// Force the database schema to a specific version.
//
// This is used to manually fix a database after a non-transactional
// migration has failed.
func (m *Command) Force(ctx context.Context, version int) error {
	return errNotImplemented
}

// Goto migrates up or down to specified version.
//
// If version is zero, then all down migrations are applied
// to result in an empty database.
func (m *Command) Goto(ctx context.Context, version int) error {
	return errNotImplemented
}

// Versions lists all of the database schema versions.
func (m *Command) Versions(ctx context.Context) ([]*Version, error) {
	return nil, errNotImplemented
}

func (m *Command) init(ctx context.Context) error {
	if m.initCalled {
		return nil
	}
	err := m.drv.CreateMigrationsTable(ctx, m.db, m.tableName())
	if err != nil {
		return err
	}
	m.initCalled = true
	return nil
}

func (m *Command) log(args ...interface{}) {
	if m.LogFunc != nil {
		m.LogFunc(args...)
	}
}

func (m *Command) transact(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return wrapf(err, "cannot begin tx")
	}

	if err = fn(tx); err != nil {
		// cannot report an error rolling back
		tx.Rollback()
		return err
	}

	if err = tx.Commit(); err != nil {
		return wrapf(err, "cannot commit tx")
	}

	return nil
}

// upOne migrates up one version using a transaction if possible.
// Reports true if there is another up migration pending at the end,
// false otherwise.
func (m *Command) upOne(ctx context.Context) (more bool, err error) {
	var (
		noTx bool
		id   int64
	)

	err = m.transact(ctx, func(tx *sql.Tx) error {
		vers, err := m.listVersions(ctx, tx)
		if err != nil {
			return err
		}

		// prepare set of version ids that have been applied
		applied := make(map[int64]struct{})
		for _, ver := range vers {
			if ver.Failed {
				return fmt.Errorf("%d: previously failed", ver.ID)
			}
			applied[ver.ID] = struct{}{}
		}

		// find list of unapplied versions, in order
		var unapplied []*migrationPlan
		for _, plan := range m.schema.plans {
			if _, ok := applied[plan.def.id]; !ok {
				// not applied yet
				unapplied = append(unapplied, plan)
			}
		}

		if len(unapplied) == 0 {
			// nothing to do
			return nil
		}

		// select the first plan
		plan := unapplied[0]
		unapplied = unapplied[1:]
		appliedAt := time.Now()
		more = len(unapplied) > 0

		if upTx := plan.def.upTx; upTx != nil {
			// Regardless of whether the driver supports transactional
			// migrations, this migration uses a transaction.
			if err = upTx(ctx, tx); err != nil {
				return wrapf(err, "%d", plan.def.id)
			}
		} else {
			if !m.drv.SupportsTransactionalDDL() || plan.def.upDB != nil {
				// Either the driver does not support transactional
				// DDL, or the up migration has been specified using
				// a non-transactional function.
				id = plan.def.id
				noTx = true
				return nil
			}
			_, err = tx.ExecContext(ctx, plan.def.upSQL)
			if err != nil {
				return wrapf(err, "%d", plan.def.id)
			}
		}

		// At this point the migration has been performed in a transaction,
		// so update the schema migrations table.
		version := &Version{
			ID:        plan.def.id,
			AppliedAt: &appliedAt,
		}

		if err = m.drv.InsertVersion(ctx, tx, m.tableName(), version); err != nil {
			return wrapf(err, "%d", plan.def.id)
		}

		return nil
	})
	if err != nil {
		return more, err
	}

	if noTx {
		// The migration needs to be performed outside of a transaction
		if err = m.upOneNoTx(ctx, id); err != nil {
			return more, err
		}
	}

	return more, nil
}

func (m *Command) upOneNoTx(ctx context.Context, id int64) error {
	var (
		err  error
		plan *migrationPlan
	)

	for _, p := range m.schema.plans {
		if p.def.id == id {
			plan = p
			break
		}
	}
	if plan == nil {
		return fmt.Errorf("missing plan for version %d", id)
	}

	// create version record with failed status
	err = m.transact(ctx, func(tx *sql.Tx) error {
		now := time.Now()
		ver := &Version{
			ID:        id,
			AppliedAt: &now,
			Failed:    true,
		}
		return m.drv.InsertVersion(ctx, tx, m.tableName(), ver)
	})
	if err != nil {
		return err
	}

	if upDB := plan.def.upDB; upDB != nil {
		if err = upDB(ctx, m.db); err != nil {
			return wrapf(err, "%d", id)
		}
	} else {
		_, err = m.db.ExecContext(ctx, plan.def.upSQL)
		if err != nil {
			return wrapf(err, "%d", id)
		}
	}

	// success, mark transaction as successful
	err = m.transact(ctx, func(tx *sql.Tx) error {
		return m.drv.SetVersionFailed(ctx, tx, m.tableName(), id, false)
	})
	if err != nil {
		return err
	}

	return nil
}

// downOne migrates down one version using a transaction if possible.
// Reports true if there is another down migration available,
// false otherwise.
func (m *Command) downOne(ctx context.Context) (more bool, err error) {
	var (
		noTx bool
		id   int64
	)

	err = m.transact(ctx, func(tx *sql.Tx) error {
		vers, err := m.listVersions(ctx, tx)
		if err != nil {
			return nil
		}

		// list of versions that have been applied, starting
		// with the first locked version
		var available []*Version
		for _, ver := range vers {
			if ver.Failed {
				return fmt.Errorf("%d: previously failed", ver.ID)
			}
			if ver.Locked {
				// clear out any previous versions, as this version
				// is locked
				available = nil
			}
			available = append(available, ver)
		}

		if len(available) == 0 {
			// nothing to do need to return status
			return nil
		}

		// select the last version
		downVer := available[len(available)-1]
		available = available[:len(available)-1]
		more = len(available) > 0

		if downVer.Locked {
			// TODO: need to return status
			return nil
		}

		var plan *migrationPlan
		for _, p := range m.schema.plans {
			if p.def.id == downVer.ID {
				plan = p
				break
			}
		}
		if plan == nil {
			return fmt.Errorf("%d: cannot find down migration", downVer.ID)
		}

		if downTx := plan.def.downTx; downTx != nil {
			// Regardless of whether the driver supports transactional
			// migrations, this migration uses a transaction.
			if err = downTx(ctx, tx); err != nil {
				return wrapf(err, "%d", plan.def.id)
			}
		} else {
			if !m.drv.SupportsTransactionalDDL() || plan.def.downDB != nil {
				// Either the driver does not support transactional
				// DDL, or the up migration has been specified using
				// a non-transactional function.
				id = plan.def.id
				noTx = true
				return nil
			}
			downSQL := plan.def.downSQL
			if downSQL == "" {
				downSQL = plan.downSQL
			}
			_, err = tx.ExecContext(ctx, downSQL)
			if err != nil {
				return wrapf(err, "%d", plan.def.id)
			}
		}

		// At this point the migration has been performed in a transaction,
		// so update the schema migrations table.
		if err = m.drv.DeleteVersion(ctx, tx, m.tableName(), downVer.ID); err != nil {
			return wrapf(err, "%d", plan.def.id)
		}

		return nil
	})
	if err != nil {
		return more, err
	}

	if noTx {
		// The migration needs to be performed outside of a transaction
		err = m.downOneNoTx(ctx, id)
	}
	return more, err
}

func (m *Command) downOneNoTx(ctx context.Context, id int64) error {
	var (
		err  error
		plan *migrationPlan
	)

	for _, p := range m.schema.plans {
		if p.def.id == id {
			plan = p
			break
		}
	}
	if plan == nil {
		return fmt.Errorf("missing plan for version %d", id)
	}

	// mark version as failed
	err = m.transact(ctx, func(tx *sql.Tx) error {
		return m.drv.SetVersionFailed(ctx, tx, m.tableName(), id, false)
	})
	if err != nil {
		return err
	}

	if downDB := plan.def.downDB; downDB != nil {
		if err = downDB(ctx, m.db); err != nil {
			return wrapf(err, "%d", id)
		}
	} else {
		downSQL := plan.def.downSQL
		if downSQL == "" {
			downSQL = plan.downSQL
		}
		_, err = m.db.ExecContext(ctx, downSQL)
		if err != nil {
			return wrapf(err, "%d", id)
		}
	}

	// success, so delete version record
	err = m.transact(ctx, func(tx *sql.Tx) error {
		return m.drv.DeleteVersion(ctx, tx, m.tableName(), id)
	})
	if err != nil {
		return err
	}

	return nil
}

func (m *Command) listVersions(ctx context.Context, tx *sql.Tx) ([]*Version, error) {
	return m.drv.ListVersions(ctx, tx, m.tableName())
}

func (m *Command) tableName() string {
	tn := m.schema.MigrationsTable
	if tn == "" {
		tn = DefaultMigrationsTable
	}
	return tn
}
