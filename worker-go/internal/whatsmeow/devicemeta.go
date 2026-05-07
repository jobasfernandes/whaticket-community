package whatsmeow

import (
	"database/sql"
	"errors"
	"fmt"
	"sync"

	_ "modernc.org/sqlite"
)

const deviceMetaSchema = `
CREATE TABLE IF NOT EXISTS device_meta (
	device_jid TEXT PRIMARY KEY,
	connection_id INTEGER NOT NULL UNIQUE
)
`

type DeviceMetaStore struct {
	mu sync.Mutex
	db *sql.DB
}

func OpenDeviceMeta(path string) (*DeviceMetaStore, error) {
	if path == "" {
		return nil, fmt.Errorf("devicemeta: path is empty")
	}
	dsn := path + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("devicemeta: open: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(deviceMetaSchema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("devicemeta: schema: %w", err)
	}
	return &DeviceMetaStore{db: db}, nil
}

func (s *DeviceMetaStore) Set(deviceJID string, connID int) error {
	if deviceJID == "" {
		return fmt.Errorf("devicemeta: deviceJID is empty")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`
		INSERT INTO device_meta(device_jid, connection_id) VALUES(?, ?)
		ON CONFLICT(device_jid) DO UPDATE SET connection_id = excluded.connection_id
	`, deviceJID, connID)
	if err != nil {
		return fmt.Errorf("devicemeta: set: %w", err)
	}
	return nil
}

func (s *DeviceMetaStore) GetConnID(deviceJID string) (int, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var connID int
	err := s.db.QueryRow(`SELECT connection_id FROM device_meta WHERE device_jid = ?`, deviceJID).Scan(&connID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("devicemeta: get conn id: %w", err)
	}
	return connID, true, nil
}

func (s *DeviceMetaStore) GetJID(connID int) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var jid string
	err := s.db.QueryRow(`SELECT device_jid FROM device_meta WHERE connection_id = ?`, connID).Scan(&jid)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("devicemeta: get jid: %w", err)
	}
	return jid, true, nil
}

func (s *DeviceMetaStore) Delete(deviceJID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.db.Exec(`DELETE FROM device_meta WHERE device_jid = ?`, deviceJID); err != nil {
		return fmt.Errorf("devicemeta: delete: %w", err)
	}
	return nil
}

func (s *DeviceMetaStore) DeleteByConnID(connID int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.db.Exec(`DELETE FROM device_meta WHERE connection_id = ?`, connID); err != nil {
		return fmt.Errorf("devicemeta: delete by conn id: %w", err)
	}
	return nil
}

func (s *DeviceMetaStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return nil
	}
	err := s.db.Close()
	s.db = nil
	return err
}
