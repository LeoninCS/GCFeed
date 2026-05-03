package infradatabase

import (
	"database/sql"
	"fmt"

	infraconfig "GCFeed/internal/infra/config"

	_ "github.com/go-sql-driver/mysql"
)

func New(dbcfg infraconfig.DatabaseConfig) (*sql.DB, error) {
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true&loc=Local",
		dbcfg.User,
		dbcfg.Password,
		dbcfg.Host,
		dbcfg.Port,
		dbcfg.Name,
	)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxIdleConns(10)
	db.SetMaxOpenConns(50)

	if err := db.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}
