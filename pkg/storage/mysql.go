package storage

import (
	"database/sql"

	_ "github.com/go-sql-driver/mysql"
)

type Mysql struct {
	db *sql.DB
}

func NewMysql(connString string) (*Mysql, error) {
	db, err := sql.Open("mysql", connString)
	if err != nil {
		return nil, err
	}

	if err = db.Ping(); err != nil {
		return nil, err
	}

	instance := Mysql{
		db: db,
	}

	if err = instance.createSchemaIfNotExists(); err != nil {
		return nil, err
	}

	return &instance, nil
}

func (s *Mysql) Apps() ([]App, error) {
	return []App{}, nil
}

func (s *Mysql) createSchemaIfNotExists() error {
	q := `
        CREATE TABLE IF NOT EXISTS test (
            id INT PRIMARY KEY AUTO_INCREMENT,
            test VARCHAR(32) DEFAULT ''
        )    
    `
	stmt, err := s.db.Prepare(q)
	if err != nil {
		return err
	}

	_, err = stmt.Exec()
	if err != nil {
		return err
	}

	return nil
}
