package main

import "time"

func (a *App) ensureProfileV273Schema() error {
	_, err := a.db.Exec(`CREATE TABLE IF NOT EXISTS user_profile_preferences_v273 (
		user_id INTEGER PRIMARY KEY,
		language TEXT NOT NULL DEFAULT 'es-419',
		timezone TEXT NOT NULL DEFAULT 'America/Bogota',
		email_notifications INTEGER NOT NULL DEFAULT 1,
		push_notifications INTEGER NOT NULL DEFAULT 1,
		theme TEXT NOT NULL DEFAULT 'light',
		updated_at TEXT NOT NULL
	)`)
	return err
}

func profileBoolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func profileUpdatedAtV273() string {
	return time.Now().UTC().Format(time.RFC3339)
}
