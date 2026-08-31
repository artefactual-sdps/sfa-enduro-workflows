// Example:
//
//  1. Make changes to schema files (internal/dips/persistence/ent/schema),
//  2. Re-generate (make gen-ent),
//  3. Use an empty MySQL database,
//  4. Run:
//     $ go run ./cmd/migrate/ \
//     --dsn="mysql://root:root123@tcp(localhost:3306)/sfa_dips_migrate" \
//     --path="./internal/dips/persistence/migrations" \
//     --name="changes"
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"ariga.io/atlas/sql/migrate"
	"ariga.io/atlas/sql/sqltool"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/sql/schema"
	"github.com/go-sql-driver/mysql"
	"github.com/spf13/pflag"

	dips_migrate "github.com/artefactual-sdps/sfa-enduro-workflows/internal/dips/persistence/ent/db/migrate"
)

func main() {
	p := pflag.NewFlagSet("migrate", pflag.ExitOnError)
	p.String("dsn", "", "MySQL DSN")
	p.String("path", "", "Migration directory")
	p.String("name", "changes", "Migration name")
	_ = p.Parse(os.Args[1:])

	DSN, _ := p.GetString("dsn")
	if DSN == "" {
		fmt.Printf("--dsn flag is missing")
		os.Exit(1)
	}

	path, _ := p.GetString("path")
	if path == "" {
		fmt.Printf("--path flag is missing")
		os.Exit(1)
	}

	// MySQL's DSN format is not accepted by Ent, convert as needed (remove Net).
	DSN = strings.TrimPrefix(DSN, "mysql://")
	DSNConfig, err := mysql.ParseDSN(DSN)
	if err != nil {
		fmt.Printf("Failed to parse MySQL DSN: %v\n", err)
		os.Exit(1)
	}
	entDSN := fmt.Sprintf("%s://%s:%s@%s/%s",
		"mysql",
		DSNConfig.User,
		DSNConfig.Passwd,
		DSNConfig.Addr,
		DSNConfig.DBName,
	)

	// Create a local migration directory able to understand golang-migrate migration files for replay.
	dir, err := sqltool.NewGolangMigrateDir(path)
	if err != nil {
		log.Fatalf("failed creating atlas migration directory: %v", err)
	}

	// Write migration diff.
	opts := []schema.MigrateOption{
		schema.WithDir(dir),                         // provide migration directory
		schema.WithMigrationMode(schema.ModeReplay), // provide migration mode
		schema.WithDialect(dialect.MySQL),           // Ent dialect to use
		schema.WithFormatter(migrate.DefaultFormatter),
		schema.WithDropIndex(true),
		schema.WithDropColumn(true),
	}

	// Append ".up" to the migration name if not already present, as a
	// workaround for the fact that Atlas omits ".up" from the ".up.sql" suffix
	// that is required when the migration is applied.
	name, _ := p.GetString("name")
	if !strings.HasSuffix(name, ".up") {
		name = fmt.Sprintf("%s.up", name)
	}

	// Generate migrations using Atlas support for MySQL (note the Ent dialect option passed above).
	if err := dips_migrate.NamedDiff(context.Background(), entDSN, name, opts...); err != nil {
		log.Fatalf("failed generating migration file: %v", err)
	}
}
