# migration: Database Migrations

[![GoDoc](https://godoc.org/github.com/jjeffery/migration?status.svg)](https://godoc.org/github.com/jjeffery/migration)
[![License](http://img.shields.io/badge/license-MIT-green.svg?style=flat)](https://raw.githubusercontent.com/jjeffery/migration/master/LICENSE.md)
[![Build Status (Linux)](https://travis-ci.org/jjeffery/migration.svg?branch=master)](https://travis-ci.org/jjeffery/migration)
[![Coverage Status](https://codecov.io/github/jjeffery/migration/badge.svg?branch=master)](https://codecov.io/github/jjeffery/migration?branch=master)
[![GoReportCard](https://goreportcard.com/badge/github.com/jjeffery/migration)](https://goreportcard.com/report/github.com/jjeffery/migration)

Package migration manages database schema migrations.

See the [Godoc](https://godoc.org/github.com/jjeffery/migration) for usage details.

## Features

* Write database migrations in SQL or Go
* Supports SQLite, Postgres and MySQL databases (support for MSSQL planned)
* Migrations are performed in a transaction where possible
* Up/Down migrations for applying and rolling back migrations
* Replay previous migrations for restoring views, functions and stored procedures
* Support for writing migrations on separate branches
* Migrations are embedded in the executable
* CLI package for easy integration with programs using [cobra](https://github.com/spf13/cobra)

## Installation
```bash
go get -u github.com/jjeffery/migration
```

## Example Usage

See the [Godoc](https://godoc.org/github.com/jjeffery/migration#example-package) package example.
