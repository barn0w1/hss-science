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

	// 1. envファイルから認証情報を読み込む (Hardcode回避)
	loadDBConfig(absEnvFile)

	fmt.Printf("🚀 Starting Deployment at: %s\n", rootDir)

	// 2. Source Get
	if err := runCmd(rootDir, "", "git", "pull", "origin", "main"); err != nil {
		log.Fatalf("Git pull failed: %v", err)
	}

	// 3. Start Postgres
	if err := runCmd(absComposeDir, "", "docker", "compose", "up", "-d", "postgres"); err != nil {
		log.Fatalf("Postgres startup failed: %v", err)
	}

	// 4. Wait
	waitForPostgres()

	// 5. Setup Databases & Migrations
	for _, db := range databases {
		absMigratePath := filepath.Join(rootDir, db.MigrationPath)
		fmt.Printf("\n--- Target DB: %s ---\n", db.Name)

		if err := createDB(absComposeDir, db.Name); err != nil {
			log.Fatalf("Create DB failed: %v", err)
		}

		if err := runMigration(absComposeDir, absMigratePath, absEnvFile, db.Name); err != nil {
			log.Fatalf("Migration failed: %v", err)
		}
	}

	// 6. Finalize
	fmt.Println("\n--- Starting All Services ---")
	if err := runCmd(absComposeDir, "", "docker", "compose", "up", "-d"); err != nil {
		log.Fatalf("Startup failed: %v", err)
	}

	fmt.Println("\n✅ Deployment finished successfully!")
}

// loadDBConfig: postgres.env を読み込んで変数にセットする
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
	if dbUser == "" || dbPassword == "" {
		log.Fatal("DB credentials not found in env file. Check POSTGRES_USER and POSTGRES_PASSWORD.")
	}
}

func createDB(workingDir, dbName string) error {
	fmt.Printf("Ensuring DB '%s' exists...\n", dbName)
	sql := fmt.Sprintf("DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = '%s') THEN CREATE DATABASE %s; END IF; END $$ ;", dbName, dbName)

	// PGPASSWORDを環境変数として渡すことで、対話式パスワード入力を回避
	return runCmd(workingDir, dbPassword, "docker", "compose", "exec", "-T", "postgres", "psql", "-U", dbUser, "-d", "postgres", "-c", sql)
}

func runMigration(workingDir, absMigratePath, absEnvFile, dbName string) error {
	fmt.Printf("Migrating %s\n", dbName)
	dbUrl := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", dbUser, dbPassword, dbHost, dbPort, dbName)

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
	log.Fatal("\nTimed out.")
}

// runCmd に password 引数を追加し、環境変数にセットできるように変更
func runCmd(dir, password string, name string, args ...string) error {
	fmt.Printf("==> Exec: %s %v\n", name, args)
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if password != "" {
		// PGPASSWORDをセットしておけばpsqlがそれを使ってくれる
		cmd.Env = append(os.Environ(), "PGPASSWORD="+password)
	}

	return cmd.Run()
}
