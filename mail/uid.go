package main

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
	"github.com/sirupsen/logrus"
)

type uidManager struct {
	db     *sql.DB
	logger *logrus.Entry
}

func createUIDManager(filename string) (*uidManager, error) {
	logger.Info("Starting UID manager")

	db, err := sql.Open("sqlite3", filename)
	if err != nil {
		return nil, err
	}

	stmt := "CREATE TABLE IF NOT EXISTS uids (id INTEGER PRIMARY KEY AUTOINCREMENT, message TEXT UNIQUE)"
	_, err = db.Exec(stmt)
	if err != nil {
		return nil, err
	}

	logger := logger.WithFields(logrus.Fields{
		"service": "uidManager",
	})

	return &uidManager{
			db:     db,
			logger: logger},
		nil
}

func (u *uidManager) Close() {
	u.db.Close()
}

func (u *uidManager) Get(id string) (uint32, error) {
	u.logger.Debug("check uid for ", id)

	fetchStmt, err := u.db.Prepare("SELECT id FROM uids WHERE message=?")
	if err != nil {
		return 0, err
	}
	defer fetchStmt.Close()
	rows, err := fetchStmt.Query(id)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	for rows.Next() {
		var uid uint32
		err = rows.Scan(&uid)
		if err != nil {
			return 0, err
		}
		u.logger.Debug("found uid ", uid, " for id for ", id)
		return uid, nil
	}
	// must insert

	tx, err := u.db.Begin()
	if err != nil {
		return 0, err
	}
	insertStmt, err := tx.Prepare("INSERT INTO uids(message) VALUES(?)")
	if err != nil {
		return 0, err
	}
	defer insertStmt.Close()
	res, err := insertStmt.Exec(id)
	if err != nil {
		return 0, err
	}
	err = tx.Commit()
	if err != nil {
		return 0, err
	}
	uid, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	u.logger.Debug("allocated uid ", uid, " for id for ", id)

	return uint32(uid), nil
}
