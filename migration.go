// Package migration manages database schema migrations.
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
// Automatically generate down migrations for views
//
// When an up migration consists of a single CREATE VIEW statement (optionally
// preceded by a DROP VIEW), then the down migration is automatically
// generated as a CREATE VIEW for the view's previous version. If no previous
// version exists, then the down migration is a DROP VIEW.
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
// part of a project-specific CLI for database management. The cli subdirectory
// contains a re-usable command line command based on the "github.com/spf13/cobra"
// package.
//
// Alternatives
//
// If this package does not meet your needs, refer to https://github.com/avelino/awesome-go#database
// for a list popular database migration packages.
package migration

import (
	"context"
	"database/sql"
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
	Version     VersionID
	Description string
}

// Error implements the error interface
func (e *Error) Error() string {
	return fmt.Sprintf("%d: %s", e.Version, e.Description)
}

// VersionID uniquely identifies a database schema version.
type VersionID int64

// Version provides information about a database schema version.
type Version struct {
	ID        VersionID  // Database schema version number
	AppliedAt *time.Time // Time migration was applied, or nil if not applied
	Failed    bool       // Did migration fail
	Locked    bool       // Is version locked (prevent down migration)
	Up        string     // SQL for up migration, or "<go-func>" if go function
	Down      string     // SQL for down migration or "<go-func>"" if a go function
}
