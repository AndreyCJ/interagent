package storage

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	_ "modernc.org/sqlite"

	"interagent/internal/port"
)

type Store struct {
	db    *sql.DB
	crypt port.Crypto
}

func New(dsn string, crypt port.Crypto) (*Store, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	s := &Store{db: db, crypt: crypt}
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
	return s.addSettingsColumns()
}

func (s *Store) addSettingsColumns() error {
	rows, err := s.db.Query(`PRAGMA table_info(settings)`)
	if err != nil {
		return err
	}
	have := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt any
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			rows.Close()
			return err
		}
		have[name] = true
	}
	rows.Close()
	for _, ddl := range []string{
		"stt_model TEXT NOT NULL DEFAULT 'base'",
		"stt_language TEXT NOT NULL DEFAULT 'auto'",
	} {
		col := ddl[:strings.Index(ddl, " ")]
		if have[col] {
			continue
		}
		if _, err := s.db.Exec(`ALTER TABLE settings ADD COLUMN ` + ddl); err != nil {
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
	if count == 0 {
		shortcuts := []port.Shortcut{
			{ID: "overlay_toggle", Label: "Show/Hide Overlay", Keys: []string{"cmd", "shift", "i"}, Enabled: true},
			{ID: "overlay_mode", Label: "Toggle click-through", Keys: []string{"cmd", "shift", "space"}, Enabled: true},
		}
		shortcutsJSON, err := json.Marshal(shortcuts)
		if err != nil {
			return err
		}
		if _, err := s.db.Exec(
			`INSERT INTO settings (id, theme, language, auto_start_listening, shortcuts, stt_model, stt_language)
			 VALUES (1, 'transparent', 'en', 0, ?, 'base', 'auto')`,
			shortcutsJSON,
		); err != nil {
			return err
		}
	}
	var agents int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM agents`).Scan(&agents); err != nil {
		return err
	}
	if agents > 0 {
		return nil
	}
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO agents (id, name, provider, model, base_url, api_key, system_prompt, temperature)
		 VALUES ('default-local', 'Local (Ollama)', 'local', 'qwen3:8b', 'http://localhost:11434', '',
		         'You are a subtle interview hint assistant. Answer concisely. Answer in the same language as the question.', 0.7)`,
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
	key := cfg.APIKey
	if key != "" {
		enc, err := s.crypt.Encrypt(key)
		if err != nil {
			return err
		}
		key = enc
	}
	_, err := s.db.Exec(
		`INSERT INTO agents (id, name, provider, model, base_url, api_key, system_prompt, temperature)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   name = excluded.name, provider = excluded.provider, model = excluded.model,
		   base_url = excluded.base_url, api_key = excluded.api_key,
		   system_prompt = excluded.system_prompt, temperature = excluded.temperature`,
		cfg.ID, cfg.Name, cfg.Provider, cfg.Model, cfg.BaseURL, key,
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
		`SELECT theme, language, auto_start_listening, shortcuts, stt_model, stt_language FROM settings WHERE id = 1`,
	).Scan(&out.Theme, &out.Language, &autoStart, &shortcuts, &out.SttModel, &out.SttLanguage)
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
		`UPDATE settings SET theme = ?, language = ?, auto_start_listening = ?, shortcuts = ?, stt_model = ?, stt_language = ?
		 WHERE id = 1`,
		cfg.Theme, cfg.Language, enabled, shortcuts, cfg.SttModel, cfg.SttLanguage,
	)
	return err
}
