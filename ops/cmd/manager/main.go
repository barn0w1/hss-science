package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type DBConfig struct {
	Name          string
	MigrationPath string
}

var (
	rootDir    string
	dbUser     string
	dbPassword string

	databases = []DBConfig{
		{
			Name:          "accounts_db",
			MigrationPath: "server/services/accounts/db/migrations",
		},
	}
)

const (
	dbHost             = "postgres"
	dbPort             = "5432"
	relativeComposeDir = "infra/envs/prod"
	networkName        = "prod_backend"
)

func init() {
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current directory: %v", err)
	}
	rootDir = cwd
}

func main() {
	absComposeDir := filepath.Join(rootDir, relativeComposeDir)
	absEnvFile := filepath.Join(absComposeDir, "envs", "postgres.env")

	loadDBConfig(absEnvFile)

	fmt.Printf("🚀 Starting Deployment at: %s\n", rootDir)

	// 1. Source Get
	fmt.Println("\n--- Updating Source Code ---")
	if err := runCmd(rootDir, "", "git", "pull", "origin", "main"); err != nil {
		log.Fatalf("Git pull failed: %v", err)
	}

	// 2. Pull Latest Images
	// これを行わないと、ローカルにある古い 'latest' イメージを使い続けてしまいます
	fmt.Println("\n--- Pulling Latest Docker Images ---")
	if err := runCmd(absComposeDir, "", "docker", "compose", "pull"); err != nil {
		log.Fatalf("Docker compose pull failed: %v", err)
	}

	// 3. Start Postgres (Migrations require DB to be up)
	fmt.Println("\n--- Starting Postgres for Migrations ---")
	if err := runCmd(absComposeDir, "", "docker", "compose", "up", "-d", "postgres"); err != nil {
		log.Fatalf("Postgres startup failed: %v", err)
	}

	// 4. Wait for Postgres health
	waitForPostgres()

	// 5. Setup Databases & Migrations
	for _, db := range databases {
		absMigratePath := filepath.Join(rootDir, db.MigrationPath)
		fmt.Printf("\n--- Target DB: %s ---\n", db.Name)

		if err := createDB(absComposeDir, db.Name); err != nil {
			log.Fatalf("Create DB failed for %s: %v", db.Name, err)
		}

		if err := runMigration(absComposeDir, absMigratePath, absEnvFile, db.Name); err != nil {
			log.Fatalf("Migration failed for %s: %v", db.Name, err)
		}
	}

	// 6. Finalize (Start/Restart all services)
	fmt.Println("\n--- Starting All Services ---")
	// --force-recreate: イメージが更新された場合、確実に新しいコンテナに入れ替える
	// --remove-orphans: composeファイルから消えた古いサービスを削除する
	if err := runCmd(absComposeDir, "", "docker", "compose", "up", "-d", "--force-recreate", "--remove-orphans"); err != nil {
		log.Fatalf("Final startup failed: %v", err)
	}

	fmt.Println("\n✅ Deployment finished successfully!")
}

func loadDBConfig(path string) {
	file, err := os.Open(path)
	if err != nil {
		log.Fatalf("Could not open env file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "POSTGRES_USER=") {
			dbUser = strings.TrimPrefix(line, "POSTGRES_USER=")
		}
		if strings.HasPrefix(line, "POSTGRES_PASSWORD=") {
			dbPassword = strings.TrimPrefix(line, "POSTGRES_PASSWORD=")
		}
	}
}

func createDB(workingDir, dbName string) error {
	fmt.Printf("Checking if DB '%s' exists...\n", dbName)

	checkSql := fmt.Sprintf("SELECT 1 FROM pg_database WHERE datname = '%s'", dbName)
	cmd := exec.Command("docker", "compose", "exec", "-T", "postgres", "psql", "-U", dbUser, "-d", "postgres", "-tAc", checkSql)
	cmd.Dir = workingDir
	cmd.Env = append(os.Environ(), "PGPASSWORD="+dbPassword)

	out, _ := cmd.Output()
	exists := strings.TrimSpace(string(out)) == "1"

	if exists {
		fmt.Printf("Database '%s' already exists. Skipping creation.\n", dbName)
		return nil
	}

	fmt.Printf("Database '%s' not found. Creating...\n", dbName)
	createSql := fmt.Sprintf("CREATE DATABASE %s", dbName)
	return runCmd(workingDir, dbPassword, "docker", "compose", "exec", "-T", "postgres", "psql", "-U", dbUser, "-d", "postgres", "-c", createSql)
}

func runMigration(workingDir, absMigratePath, absEnvFile, dbName string) error {
	fmt.Printf("Migrating %s\n", dbName)
	dbUrl := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", dbUser, dbPassword, dbHost, dbPort, dbName)

	// migration ツール自体も最新を使うためここでも pull は有効
	return runCmd(workingDir, "", "docker", "run", "--rm",
		"--network", networkName,
		"-v", fmt.Sprintf("%s:/migrations", absMigratePath),
		"--env-file", absEnvFile,
		"migrate/migrate",
		"-path", "/migrations/",
		"-database", dbUrl,
		"up",
	)
}

func waitForPostgres() {
	fmt.Print("Waiting for Postgres healthcheck...")
	for i := 0; i < 30; i++ {
		out, _ := exec.Command("docker", "inspect", "--format", "{{.State.Health.Status}}", "postgres").Output()
		if string(out) == "healthy\n" {
			fmt.Println(" OK!")
			return
		}
		fmt.Print(".")
		time.Sleep(2 * time.Second)
	}
	log.Fatal("\nTimed out waiting for Postgres.")
}

func runCmd(dir, password string, name string, args ...string) error {
	fmt.Printf("==> Exec: %s %v\n", name, args)
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if password != "" {
		cmd.Env = append(os.Environ(), "PGPASSWORD="+password)
	}

	return cmd.Run()
}
