package main

import (
	"context"
	"database/sql"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

type DatabaseConfig struct {
	dsn          string
	maxOpenConns int
	maxIdleConns int
	maxIdleTime  string
}

type database struct {
	db  *sql.DB
	cfg DatabaseConfig
}

func NewDatabase(cfg DatabaseConfig) *database {
	return &database{cfg: cfg}
}

func (db *database) DB() *sql.DB {
	return db.db
}

func (db *database) Migrate() error {
	migrationDriver, err := postgres.WithInstance(db.db, &postgres.Config{})
	if err != nil {
		return err
	}
	migrator, err := migrate.NewWithDatabaseInstance("file:///app/migrations", "postgres", migrationDriver)
	if err != nil {
		return err
	}
	err = migrator.Up()
	if err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}

func (db *database) Open() (err error) {
	db.db, err = sql.Open("mysql", db.cfg.dsn)
	if err != nil {
		return err
	}

	// Set the maximum number of open (in-use + idle) connections in the pool. Note that
	// passing a value less than or equal to 0 will mean there is no limit.
	db.db.SetMaxOpenConns(db.cfg.maxOpenConns)

	// Set the maximum number of idle connections in the pool. Again, passing a value
	// less than or equal to 0 will mean there is no limit.
	db.db.SetMaxIdleConns(db.cfg.maxIdleConns)

	duration, err := time.ParseDuration(db.cfg.maxIdleTime)
	if err != nil {
		return err
	}
	db.db.SetConnMaxIdleTime(duration)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use PingContext() to establish a new connection to the database, passing in the
	// context we created above as a parameter. If the connection couldn't be
	// established successfully within the 5 second deadline, then this will return an
	// error.
	err = db.db.PingContext(ctx)

	if err != nil {
		return err
	}

	return nil
}

func (db *database) Close() error {
	return db.db.Close()
}

func (db *database) Stats() sql.DBStats {
	return db.db.Stats()
}
