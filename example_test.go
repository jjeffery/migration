package migration_test

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"

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
	log.SetOutput(os.Stdout)

	// Perform example operations on an SQLite, in-memory database.
	ctx := context.Background()
	db, err := sql.Open("sqlite3", ":memory:")
	checkError(err)

	// A worker does the work, and can optionally log its progress.
	worker, err := migration.NewWorker(db, &Schema)
	checkError(err)
	worker.LogFunc = log.Println

	// Migrate up to the latest version
	err = worker.Up(ctx)
	checkError(err)

	// Migrate down
	err = worker.Goto(ctx, 4)
	checkError(err)

	// Output:
	// migrated up version=1
	// migrated up version=2
	// migrated up version=3
	// migrated up version=4
	// migrated up version=5
	// migrated up version=6
	// migrate up finished version=6
	// migrated down version=6
	// migrated down version=5
	// migrate goto finished version=4
}

func checkError(err error) {
	if err != nil {
		fmt.Println("error:", err)
		//log.Fatal(err)
	}
}

// init defines all of the migrations for the migration schema.
//
// In practice, the migrations would probably be defined in separate
// source files, each with its own init function.
func init() {
	Schema.Define(1).Up(`
		create table city (
			id integer not null,
			name text not null,
			countrycode character(3) not null,
			district text not null,
			population integer not null
		);
	`).Down(`drop table city;`)

	Schema.Define(2).Up(`
		create table country (
			code character(3) not null,
			name text not null,
			continent text not null,
			region text not null,
			surfacearea real not null,
			indepyear smallint,
			population integer not null,
			lifeexpectancy real,
			gnp numeric(10,2),
			gnpold numeric(10,2),
			localname text not null,
			governmentform text not null,
			headofstate text,
			capital integer,
			code2 character(2) not null
		);
	`).Down(`drop table country;`)

	Schema.Define(3).Up(`
		-- drop view first so we can replay this migration, see version 6
		drop view if exists city_country;

		create view city_country as 
			select city.id, city.name, country.name as country_name
			from city
			inner join country on city.countrycode = country.code;
	`).Down(`drop view city_country`)

	// Contrived example of a migration implemented in Go that uses
	// a database transaction.
	Schema.Define(4).UpAction(migration.TxFunc(func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
				insert into city(id, name, countrycode, district, population)
				values(?, ?, ?, ?, ?)`,
			1, "Kabul", "AFG", "Kabol", 1780000,
		)
		return err
	})).DownAction(migration.TxFunc(func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `delete from city where id = ?`, 1)
		return err
	}))

	// Contrived example of a migration implemented in Go that does
	// not use a database transaction. If this migration fails, the
	// database will require manual intervention.
	Schema.Define(5).UpAction(migration.DBFunc(func(ctx context.Context, db *sql.DB) error {
		_, err := db.ExecContext(ctx, `
				insert into city(id, name, countrycode, district, population)
				values(?, ?, ?, ?, ?)`,
			2, "Qandahar", "AFG", "Qandahar", 237500,
		)
		return err
	})).DownAction(migration.DBFunc(func(ctx context.Context, db *sql.DB) error {
		_, err := db.ExecContext(ctx, `delete from city where id = ?`, 2)
		return err
	}))

	// This migration alters the view, and the down migration reverts to the
	// previous view definition.
	Schema.Define(6).Up(`
		drop view if exists city_country;

		create view city_country as 
			select city.id, city.name, country.name as country_name, district
			from city
			inner join country on city.countrycode = country.code;
	`).DownAction(migration.Replay(3))
}
