// Package migration manages database schema migrations. This package is
// under construction and not ready for production use.
//
// There many popular packages that provide database migrations, so it is
// worth listing how this package is different.
//
// Write migrations in SQL or Go
//
// Database migrations are written in SQL using the dialect specific to the
// target database. Most of the time this is sufficient, but there are times
// when it is more convenient to specify a migration using Go code.
//
// Migrate up and migrate down
//
// Each database schema version has an "up" migration, which migrates up from
// the previous database schema version. It also has a "down" migration, which
// migrates back to the previous version.
//
// Automatically generate down migrations
//
// If an up migration consists of a single CREATE VIEW, CREATE TRIGGER, or
// CREATE PROCEDURE, then a down migration is automatically generated that will
// restore the previous version of the view, trigger or procedure.
//
// If an up migration consists of a single CREATE TABLE, CREATE INDEX, or
// CREATE DOMAIN, then a down migration is  automatically generated that will
// drop the table, index or domain.
//
// Use transactions for migrations
//
// Database migrations are performed within a transaction if the database
// supports it.
//
// Write migrations on separate branches
//
// Migration identifiers are positive 64-bit integers. Migrations can be defined
// in different VCS branches using an arbitrary naming convention, such as the
// current date and time. When branches are merged and a database migration is
// performed, any unapplied migrations will be applied in ascending order of
// identifer.
//
// Embed migrations in the executable
//
// Migrations are written as part of the Go source code, and are embedded in the
// resulting executable without the need for any embedding utility, or the need to
// deploy any separate text files.
//
// Deploy as part of a larger executable
//
// This package does not provide a stand-alone command line utility for managing
// database migrations. Instead it provides a simple API that can be utilized as
// part of a project-specific CLI for database management.
//
// Alternatives
//
// If this package does not meet your needs, refer to https://github.com/avelino/awesome-go#database
// for a list popular database migration packages.
package migration

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	// DefaultMigrationsTable is the default name of the database table
	// used to keep track of all applied database migrations. This name
	// can be overridden by the Schema.MigrationsTable field.
	DefaultMigrationsTable = "schema_migrations"
)

var (
	errNotImplemented = errors.New("not implemented")
)

// TxFunc is a function that performs a migration, either
// up to the next version or down to the previous version.
//
// The migration is performed inside a transaction, so
// if the migration fails for any reason, the database will
// rollback to its state at the start version.
type TxFunc func(context.Context, *sql.Tx) error

// DBFunc is a function that performs a migration, either up
// to the next version or down to the previous version.
//
// The migration is performed outside of a transaction, so
// if the migration fails for any reason, the database will
// require manual repair before any more migrations can proceed.
// If possible, use TxFunc to perform migrations within a
// database transaction.
type DBFunc func(context.Context, *sql.DB) error

// Errors describes one or more errors in the migration
// schema definition. If the Schema.Err() method reports a
// non-nil value, then it will be of type Errors.
type Errors []*Error

// Error implements the error interface.
func (e Errors) Error() string {
	s := make([]string, 0, len(e))

	for _, err := range e {
		s = append(s, err.Error())
	}

	return strings.TrimSpace(strings.Join(s, "\n"))
}

// Error describes a single error in the migration schema definition.
type Error struct {
	Version     int64
	Description string
}

// Error implements the error interface
func (e *Error) Error() string {
	return fmt.Sprintf("%d: %s", e.Version, e.Description)
}

// Version provides information about a database schema version.
type Version struct {
	ID      int64      // Database schema version number
	Applied *time.Time // Time migration was applied, or nil if not applied
	Failed  bool       // Did migration fail
	Up      string     // SQL for up migration, or "<go-func>" if go function
	Down    string     // SQL for down migration or "<go-func>"" if a go function
}

// A Command performs database migrations. It combines the
// information in the migration schema along with the database
// on which to perform migrations.
type Command struct {
	// LogFunc is a function for logging progress. If not specified then
	// no logging is performed.
	//
	// One common practice is to assign the log.Println function to LogFunc.
	LogFunc func(v ...interface{})

	schema *Schema
	db     *sql.DB
}

// NewCommand creates a command that can perform migrations for
// the specified dataabase using the database migration schema.
func NewCommand(db *sql.DB, schema *Schema) (*Command, error) {
	return &Command{schema: schema, db: db}, nil
}

// Up migrates the database to the latest version.
func (m *Command) Up(ctx context.Context) error {
	return errNotImplemented
}

// Version returns the current version of the database schema.
func (m *Command) Version(ctx context.Context) (*Version, error) {
	return nil, errNotImplemented
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
