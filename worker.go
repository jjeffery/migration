package migration

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"
)

// A Worker performs database migrations. It combines the
// information in the migration schema along with the database
// on which to perform migrations.
type Worker struct {
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

// NewWorker creates a worker that can perform migrations for
// the specified database using the database migration schema.
func NewWorker(db *sql.DB, schema *Schema) (*Worker, error) {
	if err := schema.Err(); err != nil {
		return nil, err
	}
	drv, err := findDriver(db)
	if err != nil {
		return nil, err
	}
	cmd := &Worker{
		schema: schema,
		db:     db,
		drv:    drv,
	}
	return cmd, nil
}

// Up migrates the database to the latest version.
func (m *Worker) Up(ctx context.Context) error {
	if err := m.init(ctx); err != nil {
		return err
	}
	for {
		more, err := m.upOne(ctx)
		if err != nil {
			return err
		}
		if !more {
			m.finished(ctx, "migrate up finished")
			break
		}
	}
	return nil
}

// Down migrates the database down to the latest locked version.
// If there are no locked versions, all down migrations are performed.
func (m *Worker) Down(ctx context.Context) error {
	if err := m.init(ctx); err != nil {
		return err
	}
	for {
		more, err := m.downOne(ctx)
		if err != nil {
			return err
		}
		if !more {
			m.finished(ctx, "migrate down finished")
			break
		}
	}
	return nil
}

// Version returns the current version of the database schema.
func (m *Worker) Version(ctx context.Context, id VersionID) (*Version, error) {
	var err error
	if err = m.checkVersion(id); err != nil {
		return nil, err
	}
	if err = m.init(ctx); err != nil {
		return nil, err
	}
	var version *Version
	err = m.transact(ctx, func(tx *sql.Tx) error {
		vs, err := m.getVersionSummaryAllowFailed(ctx, tx)
		if err != nil {
			return err
		}
		for _, ver := range vs.versions {
			if ver.ID == id {
				version = ver
				return nil
			}
		}
		return fmt.Errorf("cannot find version %d", id)
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
func (m *Worker) Force(ctx context.Context, id VersionID) error {
	var err error
	if err = m.checkVersion(id); err != nil {
		return err
	}
	if err = m.init(ctx); err != nil {
		return err
	}
	err = m.transact(ctx, func(tx *sql.Tx) error {
		vs, err := m.getVersionSummaryAllowFailed(ctx, tx)
		if err != nil {
			return err
		}
		// check for any locked versions that would prevent rolling back
		if err = vs.checkLocked(id); err != nil {
			return err
		}

		var found bool
		for _, plan := range vs.applied {
			if plan.def.id == id {
				found = true
				break
			}
		}

		if !found {
			return fmt.Errorf("cannot force unapplied version id=%d", id)
		}

		for _, ver := range vs.versions {
			if ver.ID > id {
				if err = m.drv.DeleteVersion(ctx, tx, m.tableName(), id); err != nil {
					return err
				}
				m.log(fmt.Sprintf("deleted database version id=%d", id))
			} else if ver.Failed {
				if err = m.drv.SetVersionFailed(ctx, tx, m.tableName(), id, false); err != nil {
					return err
				}
				m.log(fmt.Sprintf("cleared database version failure id=%d", id))
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

// Lock a database schema version.
//
// This is used to prevent accidental down migrations. When a database
// version is locked, it is not possible to perform a down migration to
// the previous version.
func (m *Worker) Lock(ctx context.Context, id VersionID) error {
	return m.lockHelper(ctx, id, "lock", true)
}

// Unlock a database schema version.
func (m *Worker) Unlock(ctx context.Context, id VersionID) error {
	return m.lockHelper(ctx, id, "unlock", false)
}

func (m *Worker) lockHelper(ctx context.Context, id VersionID, verb string, lock bool) error {
	var err error
	if err = m.checkVersion(id); err != nil {
		return err
	}
	if err = m.init(ctx); err != nil {
		return err
	}
	err = m.transact(ctx, func(tx *sql.Tx) error {
		vs, err := m.getVersionSummary(ctx, tx)
		if err != nil {
			return err
		}

		var found bool
		for _, plan := range vs.applied {
			if plan.def.id == id {
				found = true
				break
			}
		}

		if !found {
			return fmt.Errorf("cannot %s unapplied version id=%d", verb, id)
		}

		return m.drv.SetVersionLocked(ctx, tx, m.tableName(), id, lock)
	})
	if err != nil {
		return err
	}

	return nil
}

// Goto migrates up or down to the specified version.
//
// If id is zero, then all down migrations are applied
// to result in an empty database.
func (m *Worker) Goto(ctx context.Context, id VersionID) error {
	if err := m.checkVersion(id); err != nil {
		return err
	}
	if err := m.init(ctx); err != nil {
		return err
	}
	for {
		more, err := m.gotoOne(ctx, id)
		if err != nil {
			return err
		}
		if !more {
			m.finished(ctx, "migrate goto finished")
			break
		}
	}
	return nil
}

// Versions lists all of the database schema versions.
func (m *Worker) Versions(ctx context.Context) ([]*Version, error) {
	var versions []*Version
	if err := m.init(ctx); err != nil {
		return versions, err
	}
	err := m.transact(ctx, func(tx *sql.Tx) error {
		vs, err := m.getVersionSummary(ctx, tx)
		if err != nil {
			return err
		}
		versions = vs.versions

		return nil
	})
	return versions, err
}

func (m *Worker) init(ctx context.Context) error {
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

func (m *Worker) log(args ...interface{}) {
	if m.LogFunc != nil {
		m.LogFunc(args...)
	}
}

func (m *Worker) finished(ctx context.Context, msg string) error {
	return m.transact(ctx, func(tx *sql.Tx) error {
		vs, err := m.getVersionSummaryAllowFailed(ctx, tx)
		if err != nil {
			return err
		}
		args := []interface{}{msg}
		if len(vs.applied) > 0 {
			plan := vs.applied[0]
			version := vs.vmap[plan.def.id]
			args = append(args, fmt.Sprintf("version=%d", version.ID))
			if version.Locked {
				args = append(args, "status=locked")
			}
			if version.Failed {
				args = append(args, "status=failed")
			}
		}
		m.log(args...)
		return nil
	})
}

func (m *Worker) transact(ctx context.Context, fn func(tx *sql.Tx) error) error {
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

func (m *Worker) gotoOne(ctx context.Context, id VersionID) (more bool, err error) {
	var (
		upCount   int
		downCount int
	)
	err = m.transact(ctx, func(tx *sql.Tx) error {
		vs, err := m.getVersionSummary(ctx, tx)
		if err != nil {
			return err
		}
		// check for any locked versions that would prevent rolling back
		if err = vs.checkLocked(id); err != nil {
			return err
		}
		// count down migrations
		for _, applied := range vs.applied {
			if applied.def.id <= id {
				break
			}
			downCount++
		}

		// count up migrations
		for _, unapplied := range vs.unapplied {
			if unapplied.def.id > id {
				break
			}
			upCount++
		}

		return nil
	})
	if err != nil {
		return false, err
	}

	if downCount > 0 {
		if _, err = m.downOne(ctx); err != nil {
			return false, err
		}
		downCount--
	} else if upCount > 0 {
		if _, err = m.upOne(ctx); err != nil {
			return false, err
		}
		upCount--
	}

	more = upCount+downCount > 0
	return more, nil
}

// upOne migrates up one version using a transaction if possible.
// Reports true if there is another up migration pending at the end,
// false otherwise.
func (m *Worker) upOne(ctx context.Context) (more bool, err error) {
	var (
		noTx bool
		id   VersionID
	)

	err = m.transact(ctx, func(tx *sql.Tx) error {
		vs, err := m.getVersionSummary(ctx, tx)
		if err != nil {
			return err
		}

		if len(vs.unapplied) == 0 {
			// nothing to do
			return nil
		}

		// select the first plan
		plan := vs.unapplied[0]
		appliedAt := time.Now()
		more = len(vs.unapplied) > 1

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

		m.log(fmt.Sprintf("migrated up version=%d", plan.def.id))

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
		m.log(fmt.Sprintf("migrated up version=%d", id))
	}

	return more, nil
}

func (m *Worker) upOneNoTx(ctx context.Context, id VersionID) error {
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
func (m *Worker) downOne(ctx context.Context) (more bool, err error) {
	var (
		noTx bool
		id   VersionID
	)

	err = m.transact(ctx, func(tx *sql.Tx) error {
		vs, err := m.getVersionSummary(ctx, tx)
		if err != nil {
			return err
		}

		if len(vs.applied) == 0 {
			return nil
		}

		// the applied plan that will be reversed
		plan := vs.applied[0]
		var version *Version
		for _, ver := range vs.versions {
			if ver.ID == plan.def.id {
				version = ver
				break
			}
		}

		if version.Locked {
			m.log(fmt.Sprintf("locked version=%d", version.ID))
			return nil
		}

		more = len(vs.applied) > 1

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
		if err = m.drv.DeleteVersion(ctx, tx, m.tableName(), version.ID); err != nil {
			return wrapf(err, "%d", plan.def.id)
		}
		m.log(fmt.Sprintf("migrated down version=%d", plan.def.id))

		return nil
	})
	if err != nil {
		return more, err
	}

	if noTx {
		// The migration needs to be performed outside of a transaction
		if err = m.downOneNoTx(ctx, id); err != nil {
			return false, err
		}
		m.log(fmt.Sprintf("migrated down version=%d", id))
	}
	return more, err
}

func (m *Worker) downOneNoTx(ctx context.Context, id VersionID) error {
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

func (m *Worker) listVersions(ctx context.Context, tx *sql.Tx) ([]*Version, error) {
	return m.drv.ListVersions(ctx, tx, m.tableName())
}

func (m *Worker) tableName() string {
	tn := m.schema.MigrationsTable
	if tn == "" {
		tn = DefaultMigrationsTable
	}
	return tn
}

func (m *Worker) checkVersion(version VersionID) error {
	if _, ok := m.schema.definitions[version]; !ok {
		return fmt.Errorf("invalid schema version id=%d", version)
	}
	return nil
}

type versionSummary struct {
	id        VersionID              // highest applied version
	versions  []*Version             // applied versions, in ascending order
	applied   []*migrationPlan       // applied plans, in reverse order
	unapplied []*migrationPlan       // unapplied plans, in ascending order
	vmap      map[VersionID]*Version // map version id to version
}

func (vs *versionSummary) checkLocked(id VersionID) error {
	for _, applied := range vs.applied {
		if applied.def.id <= id {
			break
		}
		if vs.vmap[applied.def.id].Locked {
			return fmt.Errorf("database version locked id=%d", applied.def.id)
		}
	}
	return nil
}

func (m *Worker) getVersionSummary(ctx context.Context, tx *sql.Tx) (*versionSummary, error) {
	vs, err := m.getVersionSummaryAllowFailed(ctx, tx)
	if err != nil {
		return nil, err
	}
	for _, v := range vs.versions {
		if v.Failed {
			return nil, fmt.Errorf("%d: previously failed", v.ID)
		}
	}
	return vs, nil
}

func (m *Worker) getVersionSummaryAllowFailed(ctx context.Context, tx *sql.Tx) (*versionSummary, error) {
	var (
		vs  versionSummary
		err error
	)

	vs.versions, err = m.listVersions(ctx, tx)
	if err != nil {
		return nil, err
	}
	vs.vmap = make(map[VersionID]*Version)

	// prepare set of version ids that have been applied
	applied := make(map[VersionID]struct{})
	for _, ver := range vs.versions {
		if ver.ID > vs.id {
			vs.id = ver.ID
		}
		vs.vmap[ver.ID] = ver
		applied[ver.ID] = struct{}{}
	}

	// find list of unapplied versions, in order
	for _, plan := range m.schema.plans {
		var ver *Version
		if _, ok := applied[plan.def.id]; ok {
			vs.applied = append(vs.applied, plan)
			ver = vs.vmap[plan.def.id]
		} else {
			vs.unapplied = append(vs.unapplied, plan)
			ver = &Version{ID: plan.def.id}
			vs.versions = append(vs.versions, ver)
			vs.vmap[ver.ID] = ver
		}
		if plan.def.upSQL != "" {
			ver.Up = plan.def.upSQL
		} else if plan.def.upDB != nil {
			ver.Up = "(DBFunc)"
		} else if plan.def.upTx != nil {
			ver.Up = "(TxFunc)"
		}
		if plan.downSQL != "" {
			ver.Down = plan.downSQL
		} else if plan.def.downSQL != "" {
			ver.Down = plan.def.downSQL
		} else if plan.def.downDB != nil {
			ver.Down = "(DBFunc)"
		} else if plan.def.downTx != nil {
			ver.Down = "(TxFunc)"
		}
	}

	sort.Slice(vs.applied, func(i, j int) bool {
		return vs.applied[i].def.id > vs.applied[j].def.id
	})

	sort.Slice(vs.unapplied, func(i, j int) bool {
		return vs.unapplied[i].def.id < vs.unapplied[j].def.id
	})

	sort.Slice(vs.versions, func(i, j int) bool {
		return vs.versions[i].ID < vs.versions[j].ID
	})

	return &vs, nil
}