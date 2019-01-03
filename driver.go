package migration

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strings"
)

// A driver handles database vendor-specific operations.
type driver interface {
	SupportsTransactionalDDL() bool
	PackageNames() []string
	CreateMigrationsTable(ctx context.Context, db *sql.DB, tblname string) error
	InsertVersion(ctx context.Context, tx *sql.Tx, tblname string, ver *Version) error
	DeleteVersion(ctx context.Context, tx *sql.Tx, tblname string, id VersionID) error
	ListVersions(ctx context.Context, tx *sql.Tx, tblname string) ([]*Version, error)
	SetVersionFailed(ctx context.Context, tx *sql.Tx, tblname string, id VersionID, failed bool) error
	SetVersionLocked(ctx context.Context, tx *sql.Tx, tblname string, id VersionID, locked bool) error
}

var drivers = []driver{
	&postgres{},
	&sqlite{},
	&mysql{},
}

func findDriver(db *sql.DB) (driver, error) {
	driverType := reflect.TypeOf(db.Driver()).String()
	driverType = strings.TrimLeft(driverType, "*")
	split := strings.SplitN(driverType, ".", 2)
	pkgname := split[0]

	for _, drv := range drivers {
		for _, p := range drv.PackageNames() {
			if p == pkgname {
				return drv, nil
			}
		}
	}

	return nil, fmt.Errorf("cannot find migration driver for %s", pkgname)
}

type postgres struct{}

func (w *postgres) PackageNames() []string {
	return []string{"pq"}
}

func (w *postgres) SupportsTransactionalDDL() bool {
	return true
}

func (w *postgres) CreateMigrationsTable(ctx context.Context, db *sql.DB, tblname string) error {
	format := `create table if not exists %s` +
		`(id bigint primary key` +
		`,applied_at timestamptz not null` +
		`,failed boolean not null default 'false'` +
		`,locked boolean not null default 'false'` +
		`);`
	return commonCreateMigrationsTable(ctx, db, tblname, format)
}

func (w *postgres) InsertVersion(ctx context.Context, tx *sql.Tx, tblname string, ver *Version) error {
	format := `insert into %s(id,applied_at,failed,locked) values($1,$2,$3,$4);`
	return commonInsertVersion(ctx, tx, tblname, ver, format)
}

func (w *postgres) DeleteVersion(ctx context.Context, tx *sql.Tx, tblname string, id VersionID) error {
	format := `delete from %s where id = $1;`
	return commonDeleteVersion(ctx, tx, tblname, id, format)
}

func (w *postgres) ListVersions(ctx context.Context, tx *sql.Tx, tblname string) ([]*Version, error) {
	return commonListVersions(ctx, tx, tblname)
}

func (w *postgres) SetVersionFailed(ctx context.Context, tx *sql.Tx, tblname string, id VersionID, failed bool) error {
	format := `update %s set failed = $1 where id = $2`
	return commonSetBool(ctx, tx, tblname, id, failed, format)
}

func (w *postgres) SetVersionLocked(ctx context.Context, tx *sql.Tx, tblname string, id VersionID, locked bool) error {
	format := `update %s set locked = $1 where id = $2`
	return commonSetBool(ctx, tx, tblname, id, locked, format)
}

func wrapf(err error, format string, args ...interface{}) error {
	msg := fmt.Sprintf(format, args...)
	return wrappedError{Err: err, Message: msg}
}

type wrappedError struct {
	Message string
	Err     error
}

func (e wrappedError) Error() string {
	return fmt.Sprintf("%s: %v", e.Message, e.Err)
}

func (e wrappedError) Unwrap() error {
	return e.Err
}

func (e wrappedError) Cause() error {
	return e.Err
}

type sqlite struct{}

func (w *sqlite) PackageNames() []string {
	return []string{"sqlite3"}
}

func (w *sqlite) SupportsTransactionalDDL() bool {
	return true
}

func (w *sqlite) CreateMigrationsTable(ctx context.Context, db *sql.DB, tblname string) error {
	format := `create table if not exists %s` +
		`(id integer primary key` +
		`,applied_at text not null` +
		`,failed integer not null` +
		`,locked integer not null` +
		`);`
	return commonCreateMigrationsTable(ctx, db, tblname, format)
}

func (w *sqlite) InsertVersion(ctx context.Context, tx *sql.Tx, tblname string, ver *Version) error {
	format := `insert into %s(id,applied_at,failed,locked) values(?,?,?,?);`
	return commonInsertVersion(ctx, tx, tblname, ver, format)
}

func (w *sqlite) DeleteVersion(ctx context.Context, tx *sql.Tx, tblname string, id VersionID) error {
	format := `delete from %s where id = ?;`
	return commonDeleteVersion(ctx, tx, tblname, id, format)
}

func (w *sqlite) ListVersions(ctx context.Context, tx *sql.Tx, tblname string) ([]*Version, error) {
	return commonListVersions(ctx, tx, tblname)
}

func (w *sqlite) SetVersionFailed(ctx context.Context, tx *sql.Tx, tblname string, id VersionID, failed bool) error {
	format := `update %s set failed = ? where id = ?`
	return commonSetBool(ctx, tx, tblname, id, failed, format)
}

func (w *sqlite) SetVersionLocked(ctx context.Context, tx *sql.Tx, tblname string, id VersionID, locked bool) error {
	format := `update %s set locked = ? where id = ?`
	return commonSetBool(ctx, tx, tblname, id, locked, format)
}

type mysql struct{}

func (w *mysql) PackageNames() []string {
	return []string{"mysql"}
}

func (w *mysql) SupportsTransactionalDDL() bool {
	return false
}

func (w *mysql) CreateMigrationsTable(ctx context.Context, db *sql.DB, tblname string) error {
	format := `create table if not exists %s` +
		`(id integer primary key` +
		`,applied_at datetime not null` +
		`,failed integer not null` +
		`,locked integer not null` +
		`);`
	return commonCreateMigrationsTable(ctx, db, tblname, format)
}

func (w *mysql) InsertVersion(ctx context.Context, tx *sql.Tx, tblname string, ver *Version) error {
	format := `insert into %s(id,applied_at,failed,locked) values(?,?,?,?);`
	return commonInsertVersion(ctx, tx, tblname, ver, format)
}

func (w *mysql) DeleteVersion(ctx context.Context, tx *sql.Tx, tblname string, id VersionID) error {
	format := `delete from %s where id = ?;`
	return commonDeleteVersion(ctx, tx, tblname, id, format)
}

func (w *mysql) ListVersions(ctx context.Context, tx *sql.Tx, tblname string) ([]*Version, error) {
	return commonListVersions(ctx, tx, tblname)
}

func (w *mysql) SetVersionFailed(ctx context.Context, tx *sql.Tx, tblname string, id VersionID, failed bool) error {
	format := `update %s set failed = ? where id = ?`
	return commonSetBool(ctx, tx, tblname, id, failed, format)
}

func (w *mysql) SetVersionLocked(ctx context.Context, tx *sql.Tx, tblname string, id VersionID, locked bool) error {
	format := `update %s set locked = ? where id = ?`
	return commonSetBool(ctx, tx, tblname, id, locked, format)
}

func commonCreateMigrationsTable(ctx context.Context, db *sql.DB, tblname string, format string) error {
	query := fmt.Sprintf(format, tblname)
	_, err := db.ExecContext(ctx, query)
	if err != nil {
		return wrapf(err, "cannot create table %s", tblname)
	}
	return nil
}

func commonInsertVersion(ctx context.Context, tx *sql.Tx, tblname string, ver *Version, format string) error {
	query := fmt.Sprintf(format, tblname)
	_, err := tx.ExecContext(ctx, query, ver.ID, *ver.AppliedAt, ver.Failed, ver.Locked)
	if err != nil {
		return wrapf(err, "cannot insert migration version %d", ver.ID)
	}
	return nil
}

func commonDeleteVersion(ctx context.Context, tx *sql.Tx, tblname string, id VersionID, format string) error {
	query := fmt.Sprintf(format, tblname)
	_, err := tx.ExecContext(ctx, query, id)
	if err != nil {
		return wrapf(err, "cannot delete migration version %d", id)
	}
	return nil
}

func commonSetBool(ctx context.Context, tx *sql.Tx, tblname string, id VersionID, boolval bool, format string) error {
	query := fmt.Sprintf(format, tblname)
	_, err := tx.ExecContext(ctx, query, boolval, id)
	if err != nil {
		return wrapf(err, "cannot update migration version %d", id)
	}
	return nil
}

func commonListVersions(ctx context.Context, tx *sql.Tx, tblname string) ([]*Version, error) {
	var versions []*Version
	format := `select id,applied_at,failed,locked from %s order by id`
	query := fmt.Sprintf(format, tblname)
	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		return nil, wrapf(err, "cannot query versions")
	}
	for rows.Next() {
		var (
			ver       Version
			appliedAt timeVal
		)

		if err = rows.Scan(&ver.ID, &appliedAt, &ver.Failed, &ver.Locked); err != nil {
			return nil, wrapf(err, "cannot scan version")
		}
		ver.AppliedAt = &appliedAt.Time
		versions = append(versions, &ver)
	}
	if err = rows.Err(); err != nil {
		return nil, wrapf(err, "cannot scan versions")
	}

	return versions, nil
}
