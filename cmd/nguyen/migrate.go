package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/dev2k6/Nguyen.go/internal/config"
	"github.com/dev2k6/Nguyen.go/internal/database"
	"github.com/spf13/cobra"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Database migration management",
}

var migrateUpCmd = &cobra.Command{
	Use:   "up",
	Short: "Run all pending migrations",
	RunE: func(cmd *cobra.Command, args []string) error {
		db, migrator, err := setupMigrator()
		if err != nil {
			return err
		}
		defer db.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		return migrator.Up(ctx)
	},
}

var migrateDownCmd = &cobra.Command{
	Use:   "down [steps]",
	Short: "Rollback migrations",
	RunE: func(cmd *cobra.Command, args []string) error {
		db, migrator, err := setupMigrator()
		if err != nil {
			return err
		}
		defer db.Close()

		steps := 1
		if len(args) > 0 {
			fmt.Sscanf(args[0], "%d", &steps)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		return migrator.Down(ctx, steps)
	},
}

var migrateStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show migration status",
	RunE: func(cmd *cobra.Command, args []string) error {
		db, migrator, err := setupMigrator()
		if err != nil {
			return err
		}
		defer db.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		migrations, err := migrator.Status(ctx)
		if err != nil {
			return err
		}

		if len(migrations) == 0 {
			fmt.Println("  No migrations applied yet")
			return nil
		}

		fmt.Println("  Applied migrations:")
		for _, m := range migrations {
			fmt.Printf("    %s_%s (applied: %s)\n", m.Version, m.Name, m.AppliedAt.Format("2006-01-02 15:04:05"))
		}
		return nil
	},
}

var migrateCreateCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a new migration file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load("config/nguyen.config.yml")
		if err != nil {
			cfg = config.DefaultConfig()
		}

		dir := cfg.Database.Migrations
		if dir == "" {
			dir = "migrations"
		}

		migrator := database.NewMigrator(nil, dir)
		name, err := migrator.Create(args[0])
		if err != nil {
			return err
		}

		fmt.Printf("  Created migration: %s\n", name)
		fmt.Printf("    %s/%s.up.sql\n", dir, name)
		fmt.Printf("    %s/%s.down.sql\n", dir, name)
		return nil
	},
}

func init() {
	migrateCmd.AddCommand(migrateUpCmd)
	migrateCmd.AddCommand(migrateDownCmd)
	migrateCmd.AddCommand(migrateStatusCmd)
	migrateCmd.AddCommand(migrateCreateCmd)
	RootCmd.AddCommand(migrateCmd)
}

func setupMigrator() (*database.DB, *database.Migrator, error) {
	cfg, err := config.Load("config/nguyen.config.yml")
	if err != nil {
		cfg = config.DefaultConfig()
	}

	if cfg.Database.Driver == "" || cfg.Database.DSN == "" {
		return nil, nil, fmt.Errorf("database configuration is required in config/nguyen.config.yml")
	}

	db, err := database.New(database.Config{
		Driver:      cfg.Database.Driver,
		DSN:         cfg.Database.DSN,
		MaxOpenConn: cfg.Database.MaxOpenConn,
		MaxIdleConn: cfg.Database.MaxIdleConn,
		MaxLifetime: cfg.Database.MaxLifetime,
	})
	if err != nil {
		return nil, nil, err
	}

	dir := cfg.Database.Migrations
	if dir == "" {
		dir = "migrations"
	}

	migrator := database.NewMigrator(db, dir)
	log.Println("  Connected to database")
	return db, migrator, nil
}
