package migration_test

import (
	"context"
	"database/sql"
	"log"

	"github.com/jjeffery/migration"
	_ "github.com/mattn/go-sqlite3"
)

// Schema contains all the information needed to migrate
// the database schema.
//
// See the init function  below for where the individual
// migrations are defined.
var Schema migration.Schema

func Example() {
	// Setup logging. Don't print a timestamp so that the
	// output can be checked at the end of this function.
	log.SetFlags(0)

	// Perform example operations on an SQLite, in-memory database.
	ctx := context.Background()
	db, err := sql.Open("sqlite3", ":memory")
	checkError(err)

	// A command does the work, and can optionally log its progress.
	command, err := migration.NewCommand(db, &Schema)
	checkError(err)
	command.LogFunc = log.Println

	// Migrate up to the latest version
	err = command.Up(ctx)
	checkError(err)

	// Migrate down
	err = command.Goto(ctx, 4)
	checkError(err)

	// Report database schema version
	version, err := command.Version(ctx)
	checkError(err)
	log.Printf("database schema version=%d", version.ID)

	// __Output:
	// migrated up version=1
	// migrated up version=2
	// migrated up version=3
	// migrated up version=4
	// migrated up version=5
	// migrated down version=5
	// database schema version=4
}

func checkError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

// init defines all of the migrations for the migration schema.
//
// In practice, the migrations would probably be defined in separate
// source files, each with its own init function.
func init() {
	// Version 1: note that down migration is automatically inferred.
	Schema.Define(1).Up(`
		CREATE TABLE city (
			id integer NOT NULL,
			name text NOT NULL,
			countrycode character(3) NOT NULL,
			district text NOT NULL,
			population integer NOT NULL
		);
	`)

	// Version 2: down migration is automatically inferred.
	Schema.Define(2).Up(`
		CREATE TABLE country (
			code character(3) NOT NULL,
			name text NOT NULL,
			continent text NOT NULL,
			region text NOT NULL,
			surfacearea real NOT NULL,
			indepyear smallint,
			population integer NOT NULL,
			lifeexpectancy real,
			gnp numeric(10,2),
			gnpold numeric(10,2),
			localname text NOT NULL,
			governmentform text NOT NULL,
			headofstate text,
			capital integer,
			code2 character(2) NOT NULL
		);
	`)

	// Version 3: down migration is provided
	Schema.Define(3).Up(`
		CREATE TABLE countrylanguage (
			countrycode character(3) NOT NULL,
			"language" text NOT NULL,
			isofficial boolean NOT NULL,
			percentage real NOT NULL
		);
	`).Down(`
		-- this down migration is not necessary, as the migration package
		-- can work it out from the up SQL
		DROP TABLE countrylanguage
	`)

	// Contrived example of a migration implemented in Go that uses
	// a database transaction.
	Schema.Define(4).
		UpTx(
			func(ctx context.Context, tx *sql.Tx) error {
				_, err := tx.ExecContext(ctx, `
				insert into city(id, name, countrycode, district, population)
				values(?, ?, ?, ?, ?)`,
					1, "Kabul", "AFG", "Kabol", 1780000,
				)
				return err
			},
		).
		DownTx(
			func(ctx context.Context, tx *sql.Tx) error {
				_, err := tx.ExecContext(ctx, `delete from city where id = ?`, 1)
				return err
			},
		)

	// Contrived example of a migration implemented in Go that does
	// not use a database transaction. If this migration fails, the
	// database will require manual intervention.
	Schema.Define(5).
		UpDB(
			func(ctx context.Context, db *sql.DB) error {
				_, err := db.ExecContext(ctx, `
				insert into city(id, name, countrycode, district, population)
				values(?, ?, ?, ?, ?)`,
					2, "Qandahar", "AFG", "Qandahar", 237500,
				)
				return err
			},
		).
		DownDB(
			func(ctx context.Context, db *sql.DB) error {
				_, err := db.ExecContext(ctx, `delete from city where id = ?`, 2)
				return err
			},
		)
}
