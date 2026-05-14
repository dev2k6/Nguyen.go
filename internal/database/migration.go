package database

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Migration struct {
	Version   string
	Name      string
	UpSQL     string
	DownSQL   string
	AppliedAt time.Time
}

type Migrator struct {
	db        *DB
	dir       string
	tableName string
}

func NewMigrator(db *DB, dir string) *Migrator {
	return &Migrator{
		db:        db,
		dir:       dir,
		tableName: "nguyen_migrations",
	}
}

func (m *Migrator) Init(ctx context.Context) error {
	query := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		version VARCHAR(255) PRIMARY KEY,
		name VARCHAR(255) NOT NULL,
		applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`, m.tableName)

	_, err := m.db.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("database: failed to create migrations table: %w", err)
	}
	return nil
}

func (m *Migrator) Up(ctx context.Context) error {
	if err := m.Init(ctx); err != nil {
		return err
	}

	migrations, err := m.pending(ctx)
	if err != nil {
		return err
	}

	if len(migrations) == 0 {
		log.Println("  database: no pending migrations")
		return nil
	}

	for _, mig := range migrations {
		if err := m.apply(ctx, mig); err != nil {
			return fmt.Errorf("database: migration %s failed: %w", mig.Version, err)
		}
		log.Printf("  database: applied migration %s_%s\n", mig.Version, mig.Name)
	}

	return nil
}

func (m *Migrator) Down(ctx context.Context, steps int) error {
	if err := m.Init(ctx); err != nil {
		return err
	}

	applied, err := m.applied(ctx)
	if err != nil {
		return err
	}

	if steps <= 0 || steps > len(applied) {
		steps = len(applied)
	}

	for i := len(applied) - 1; i >= len(applied)-steps; i-- {
		mig := applied[i]
		downSQL, err := m.readFile(mig.Version, mig.Name, "down")
		if err != nil {
			return err
		}
		mig.DownSQL = downSQL

		if err := m.rollback(ctx, mig); err != nil {
			return fmt.Errorf("database: rollback %s failed: %w", mig.Version, err)
		}
		log.Printf("  database: rolled back migration %s_%s\n", mig.Version, mig.Name)
	}

	return nil
}

func (m *Migrator) Status(ctx context.Context) ([]Migration, error) {
	if err := m.Init(ctx); err != nil {
		return nil, err
	}
	return m.applied(ctx)
}

func (m *Migrator) Create(name string) (string, error) {
	version := time.Now().Format("20060102150405")
	baseName := fmt.Sprintf("%s_%s", version, name)

	upPath := filepath.Join(m.dir, baseName+".up.sql")
	downPath := filepath.Join(m.dir, baseName+".down.sql")

	if err := os.MkdirAll(m.dir, 0755); err != nil {
		return "", fmt.Errorf("database: cannot create migrations dir: %w", err)
	}

	if err := os.WriteFile(upPath, []byte("-- Write your UP migration here\n"), 0644); err != nil {
		return "", fmt.Errorf("database: cannot create up file: %w", err)
	}
	if err := os.WriteFile(downPath, []byte("-- Write your DOWN migration here\n"), 0644); err != nil {
		return "", fmt.Errorf("database: cannot create down file: %w", err)
	}

	return baseName, nil
}

func (m *Migrator) pending(ctx context.Context) ([]Migration, error) {
	applied, err := m.applied(ctx)
	if err != nil {
		return nil, err
	}

	appliedMap := make(map[string]bool)
	for _, a := range applied {
		appliedMap[a.Version] = true
	}

	all, err := m.discover()
	if err != nil {
		return nil, err
	}

	var pending []Migration
	for _, mig := range all {
		if !appliedMap[mig.Version] {
			pending = append(pending, mig)
		}
	}
	return pending, nil
}

func (m *Migrator) applied(ctx context.Context) ([]Migration, error) {
	query := fmt.Sprintf("SELECT version, name, applied_at FROM %s ORDER BY version ASC", m.tableName)
	rows, err := m.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("database: cannot read applied migrations: %w", err)
	}
	defer rows.Close()

	var migrations []Migration
	for rows.Next() {
		var mig Migration
		if err := rows.Scan(&mig.Version, &mig.Name, &mig.AppliedAt); err != nil {
			return nil, fmt.Errorf("database: scan error: %w", err)
		}
		migrations = append(migrations, mig)
	}
	return migrations, rows.Err()
}

func (m *Migrator) discover() ([]Migration, error) {
	entries, err := os.ReadDir(m.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("database: cannot read migrations dir: %w", err)
	}

	seen := make(map[string]*Migration)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}

		base := strings.TrimSuffix(name, ".up.sql")
		parts := strings.SplitN(base, "_", 2)
		if len(parts) != 2 {
			continue
		}

		version := parts[0]
		migName := parts[1]

		upSQL, err := os.ReadFile(filepath.Join(m.dir, name))
		if err != nil {
			return nil, fmt.Errorf("database: cannot read %s: %w", name, err)
		}

		seen[version] = &Migration{
			Version: version,
			Name:    migName,
			UpSQL:   string(upSQL),
		}
	}

	var migrations []Migration
	for _, mig := range seen {
		migrations = append(migrations, *mig)
	}
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})
	return migrations, nil
}

func (m *Migrator) readFile(version, name, direction string) (string, error) {
	filename := fmt.Sprintf("%s_%s.%s.sql", version, name, direction)
	data, err := os.ReadFile(filepath.Join(m.dir, filename))
	if err != nil {
		return "", fmt.Errorf("database: cannot read %s: %w", filename, err)
	}
	return string(data), nil
}

func (m *Migrator) apply(ctx context.Context, mig Migration) error {
	tx, err := m.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, mig.UpSQL); err != nil {
		return err
	}

	insert := fmt.Sprintf("INSERT INTO %s (version, name) VALUES ($1, $2)", m.tableName)
	if _, err := tx.ExecContext(ctx, insert, mig.Version, mig.Name); err != nil {
		return err
	}

	return tx.Commit()
}

func (m *Migrator) rollback(ctx context.Context, mig Migration) error {
	tx, err := m.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, mig.DownSQL); err != nil {
		return err
	}

	del := fmt.Sprintf("DELETE FROM %s WHERE version = $1", m.tableName)
	if _, err := tx.ExecContext(ctx, del, mig.Version); err != nil {
		return err
	}

	return tx.Commit()
}
