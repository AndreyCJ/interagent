package storage

import (
	"database/sql"
	"encoding/json"
	"errors"

	_ "modernc.org/sqlite"

	"interagent/internal/port"
)

type Store struct{ db *sql.DB }

func New(dsn string) (*Store, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := s.seed(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			chat_history TEXT NOT NULL,
			started_at INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS agents (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			provider TEXT NOT NULL,
			model TEXT NOT NULL,
			base_url TEXT NOT NULL DEFAULT '',
			api_key TEXT NOT NULL DEFAULT '',
			system_prompt TEXT NOT NULL DEFAULT '',
			temperature REAL NOT NULL DEFAULT 0.7
		)`,
		`CREATE TABLE IF NOT EXISTS settings (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			theme TEXT NOT NULL,
			language TEXT NOT NULL,
			auto_start_listening INTEGER NOT NULL,
			shortcuts TEXT NOT NULL
		)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) seed() error {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM settings`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	shortcuts := []port.Shortcut{
		{ID: "overlay_toggle", Label: "Show/Hide Overlay", Keys: []string{"cmd", "shift", "i"}, Enabled: true},
		{ID: "overlay_mode", Label: "Toggle click-through", Keys: []string{"cmd", "shift", "space"}, Enabled: true},
	}
	shortcutsJSON, err := json.Marshal(shortcuts)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(
		`INSERT INTO settings (id, theme, language, auto_start_listening, shortcuts)
		 VALUES (1, 'transparent', 'en', 0, ?)`,
		shortcutsJSON,
	)
	return err
}

// --- Sessions ---

func (s *Store) CreateSession(sess port.Session) error {
	historyJSON, err := json.Marshal(sess.ChatHistory)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(
		`INSERT INTO sessions (id, chat_history, started_at) VALUES (?, ?, ?)`,
		sess.ID, historyJSON, sess.StartedAt,
	)
	return err
}

func (s *Store) GetSession(id string) (port.Session, error) {
	var (
		sess   port.Session
		rawStr string
	)
	err := s.db.QueryRow(
		`SELECT id, chat_history, started_at FROM sessions WHERE id = ?`, id,
	).Scan(&sess.ID, &rawStr, &sess.StartedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return port.Session{}, errors.New("session not found")
		}
		return port.Session{}, err
	}
	if err := json.Unmarshal([]byte(rawStr), &sess.ChatHistory); err != nil {
		return port.Session{}, err
	}
	return sess, nil
}

func (s *Store) UpdateSession(sess port.Session) error {
	historyJSON, err := json.Marshal(sess.ChatHistory)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(
		`UPDATE sessions SET chat_history = ?, started_at = ? WHERE id = ?`,
		historyJSON, sess.StartedAt, sess.ID,
	)
	return err
}

func (s *Store) DeleteSession(id string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE id = ?`, id)
	return err
}

// --- Agents ---

func (s *Store) GetAgents() ([]port.AgentConfig, error) {
	rows, err := s.db.Query(
		`SELECT id, name, provider, model, base_url, api_key, system_prompt, temperature
		 FROM agents ORDER BY name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	agents := []port.AgentConfig{}
	for rows.Next() {
		var a port.AgentConfig
		if err := rows.Scan(&a.ID, &a.Name, &a.Provider, &a.Model, &a.BaseURL,
			&a.APIKey, &a.SystemPrompt, &a.Temperature); err != nil {
			return nil, err
		}
		agents = append(agents, a)
	}
	return agents, rows.Err()
}

func (s *Store) SaveAgent(cfg port.AgentConfig) error {
	_, err := s.db.Exec(
		`INSERT INTO agents (id, name, provider, model, base_url, api_key, system_prompt, temperature)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   name = excluded.name, provider = excluded.provider, model = excluded.model,
		   base_url = excluded.base_url, api_key = excluded.api_key,
		   system_prompt = excluded.system_prompt, temperature = excluded.temperature`,
		cfg.ID, cfg.Name, cfg.Provider, cfg.Model, cfg.BaseURL, cfg.APIKey,
		cfg.SystemPrompt, cfg.Temperature,
	)
	return err
}

func (s *Store) DeleteAgent(id string) error {
	_, err := s.db.Exec(`DELETE FROM agents WHERE id = ?`, id)
	return err
}

// --- Settings ---

func (s *Store) GetSettings() (port.AppSettings, error) {
	var (
		out       port.AppSettings
		shortcuts string
		autoStart int
	)
	err := s.db.QueryRow(
		`SELECT theme, language, auto_start_listening, shortcuts FROM settings WHERE id = 1`,
	).Scan(&out.Theme, &out.Language, &autoStart, &shortcuts)
	if err != nil {
		return port.AppSettings{}, err
	}
	out.AutoStartListening = autoStart != 0
	if err := json.Unmarshal([]byte(shortcuts), &out.Shortcuts); err != nil {
		return port.AppSettings{}, err
	}
	return out, nil
}

func (s *Store) SaveSettings(cfg port.AppSettings) error {
	enabled := 0
	if cfg.AutoStartListening {
		enabled = 1
	}
	shortcuts, err := json.Marshal(cfg.Shortcuts)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(
		`UPDATE settings SET theme = ?, language = ?, auto_start_listening = ?, shortcuts = ?
		 WHERE id = 1`,
		cfg.Theme, cfg.Language, enabled, shortcuts,
	)
	return err
}
