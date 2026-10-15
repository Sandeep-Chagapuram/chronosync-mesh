package mysql_driver

import (
	"chronosync-mesh/config"
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
)

type MySQLClient struct {
	DB *sql.DB
}

func NewMySQLClient(cfg config.DBDetails) (*MySQLClient, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(cfg.PoolSize)
	db.SetMaxIdleConns(cfg.PoolSize / 2)

	if err := db.Ping(); err != nil {
		return nil, err
	}

	return &MySQLClient{DB: db}, nil
}
