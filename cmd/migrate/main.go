package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

func qi(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }

func main() {
	source := flag.String("source", env("SQLITE_SOURCE", "/var/data/worktic.db"), "ruta SQLite origen")
	target := flag.String("target", env("DATABASE_URL", ""), "DATABASE_URL PostgreSQL destino")
	dry := flag.Bool("dry-run", false, "solo mostrar tablas")
	flag.Parse()
	if strings.TrimSpace(*target) == "" {
		log.Fatal("falta --target o DATABASE_URL")
	}
	if _, err := os.Stat(*source); err != nil {
		log.Fatalf("SQLite origen no disponible: %v", err)
	}

	src, err := sql.Open("sqlite", "file:"+*source+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		log.Fatal(err)
	}
	defer src.Close()
	dst, err := sql.Open("postgres", *target)
	if err != nil {
		log.Fatal(err)
	}
	defer dst.Close()
	if err = src.Ping(); err != nil {
		log.Fatalf("SQLite: %v", err)
	}
	if err = dst.Ping(); err != nil {
		log.Fatalf("PostgreSQL: %v", err)
	}

	sourceTables, err := sqliteTables(src)
	if err != nil {
		log.Fatal(err)
	}
	destTables, err := postgresTables(dst)
	if err != nil {
		log.Fatal(err)
	}
	skip := map[string]bool{"app_sessions": true, "security_audit_v26": true, "production_migrations_v26": true}
	var common, sourceOnly []string
	for _, t := range sourceTables {
		if skip[t] || strings.HasPrefix(t, "sqlite_") {
			continue
		}
		if destTables[t] {
			common = append(common, t)
		} else {
			sourceOnly = append(sourceOnly, t)
		}
	}
	sort.Strings(common)
	sort.Strings(sourceOnly)
	log.Printf("tablas compatibles encontradas: %d", len(common))
	if len(sourceOnly) > 0 {
		log.Printf("tablas solo en SQLite (no se copian; normalmente stores QR/legacy): %s", strings.Join(sourceOnly, ", "))
	}
	if *dry {
		for _, t := range common {
			if err := validateTableCompatibility(src, dst, t); err != nil {
				log.Fatalf("dry-run %s: %v", t, err)
			}
			fmt.Println("OK", t)
		}
		log.Printf("dry-run OK: %d tablas compatibles; no se modificó PostgreSQL", len(common))
		return
	}

	started := time.Now()
	if err := truncateDestination(dst, common); err != nil {
		log.Fatalf("limpiando destino PostgreSQL: %v", err)
	}
	totalRows := int64(0)
	for _, table := range common {
		n, err := copyTable(src, dst, table)
		if err != nil {
			log.Fatalf("migrando %s: %v", table, err)
		}
		totalRows += n
		log.Printf("%-40s %d filas", table, n)
	}
	if err := verifyCounts(src, dst, common); err != nil {
		log.Fatalf("verificación post-migración: %v", err)
	}
	_, _ = dst.Exec(`INSERT INTO production_migrations_v26(migration_key,status,details,applied_at) VALUES($1,$2,$3,$4) ON CONFLICT(migration_key) DO UPDATE SET status=EXCLUDED.status,details=EXCLUDED.details,applied_at=EXCLUDED.applied_at`, "sqlite_to_postgres_v26", "completed", fmt.Sprintf("tables=%d rows=%d source=%s", len(common), totalRows, *source), time.Now().UTC().Format(time.RFC3339))
	log.Printf("migración completada: %d tablas / %d filas / %s", len(common), totalRows, time.Since(started).Round(time.Millisecond))
	log.Printf("IMPORTANTE: app_sessions no se migró; todos los usuarios deberán iniciar sesión de nuevo")
}

func env(k, d string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return d
}

func sqliteTables(db *sql.DB) ([]string, error) {
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var n string
		if rows.Scan(&n) == nil {
			out = append(out, n)
		}
	}
	return out, rows.Err()
}
func postgresTables(db *sql.DB) (map[string]bool, error) {
	rows, err := db.Query(`SELECT table_name FROM information_schema.tables WHERE table_schema='public' AND table_type='BASE TABLE'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var n string
		if rows.Scan(&n) == nil {
			out[n] = true
		}
	}
	return out, rows.Err()
}
func sqliteColumns(db *sql.DB, table string) ([]string, error) {
	rows, err := db.Query(`PRAGMA table_info(` + qi(table) + `)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull, pk int
		var def any
		if err := rows.Scan(&cid, &name, &typ, &notnull, &def, &pk); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}
func postgresColumns(db *sql.DB, table string) (map[string]bool, error) {
	rows, err := db.Query(`SELECT column_name FROM information_schema.columns WHERE table_schema='public' AND table_name=$1`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var n string
		if rows.Scan(&n) == nil {
			out[n] = true
		}
	}
	return out, rows.Err()
}
func validateTableCompatibility(src, dst *sql.DB, table string) error {
	sc, err := sqliteColumns(src, table)
	if err != nil {
		return err
	}
	dc, err := postgresColumns(dst, table)
	if err != nil {
		return err
	}
	var missing []string
	for _, c := range sc {
		if !dc[c] {
			missing = append(missing, c)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("columnas del origen ausentes en PostgreSQL: %s", strings.Join(missing, ", "))
	}
	return nil
}

func truncateDestination(dst *sql.DB, tables []string) error {
	if len(tables) == 0 {
		return nil
	}
	quoted := make([]string, 0, len(tables))
	for _, table := range tables {
		quoted = append(quoted, qi(table))
	}
	_, err := dst.Exec(`TRUNCATE TABLE ` + strings.Join(quoted, ",") + ` RESTART IDENTITY CASCADE`)
	return err
}

func verifyCounts(src, dst *sql.DB, tables []string) error {
	for _, table := range tables {
		var sourceCount, destCount int64
		if err := src.QueryRow(`SELECT COUNT(*) FROM ` + qi(table)).Scan(&sourceCount); err != nil {
			return fmt.Errorf("%s origen: %w", table, err)
		}
		if err := dst.QueryRow(`SELECT COUNT(*) FROM ` + qi(table)).Scan(&destCount); err != nil {
			return fmt.Errorf("%s destino: %w", table, err)
		}
		if sourceCount != destCount {
			return fmt.Errorf("%s no coincide: SQLite=%d PostgreSQL=%d", table, sourceCount, destCount)
		}
	}
	log.Printf("verificación de conteos OK para %d tablas", len(tables))
	return nil
}

func copyTable(src, dst *sql.DB, table string) (int64, error) {
	sc, err := sqliteColumns(src, table)
	if err != nil {
		return 0, err
	}
	dc, err := postgresColumns(dst, table)
	if err != nil {
		return 0, err
	}
	var cols []string
	var missing []string
	for _, c := range sc {
		if dc[c] {
			cols = append(cols, c)
		} else {
			missing = append(missing, c)
		}
	}
	if len(missing) > 0 {
		return 0, fmt.Errorf("columnas del origen ausentes en PostgreSQL: %s", strings.Join(missing, ", "))
	}
	if len(cols) == 0 {
		return 0, nil
	}
	tx, err := dst.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	qcols := make([]string, len(cols))
	for i, c := range cols {
		qcols[i] = qi(c)
	}
	rows, err := src.Query(`SELECT ` + strings.Join(qcols, ",") + ` FROM ` + qi(table))
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	ph := make([]string, len(cols))
	for i := range ph {
		ph[i] = fmt.Sprintf("$%d", i+1)
	}
	ins := `INSERT INTO ` + qi(table) + ` (` + strings.Join(qcols, ",") + `) VALUES (` + strings.Join(ph, ",") + ") ON CONFLICT DO NOTHING"
	stmt, err := tx.Prepare(ins)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()
	var count int64
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err = rows.Scan(ptrs...); err != nil {
			return count, err
		}
		if _, err = stmt.Exec(vals...); err != nil {
			return count, err
		}
		count++
	}
	if err = rows.Err(); err != nil {
		return count, err
	}
	if dc["id"] {
		_, _ = tx.Exec(`SELECT setval(pg_get_serial_sequence($1,'id'), COALESCE((SELECT MAX(id) FROM `+qi(table)+`),1), COALESCE((SELECT MAX(id) FROM `+qi(table)+`),0)>0)`, table)
	}
	if err = tx.Commit(); err != nil {
		return count, err
	}
	return count, nil
}
