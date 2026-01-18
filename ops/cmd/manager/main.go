package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"
)

func run(name string, args ...string) error {
	fmt.Printf("==> Executing: %s %v\n", name, args)
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = "infra/envs/prod"
	return cmd.Run()
}

func main() {
	fmt.Println("Starting Deployment...")

	// 1. Source Get
	// 注: cmd.Dir の外(リポジトリルート)で実行する必要があるため、一時的に空
	gitPull := exec.Command("git", "pull", "origin", "main")
	gitPull.Stdout = os.Stdout
	gitPull.Stderr = os.Stderr
	if err := gitPull.Run(); err != nil {
		log.Fatalf("Git pull failed: %v", err)
	}

	// 2. DB起動
	if err := run("docker", "compose", "up", "-d", "postgres"); err != nil {
		log.Fatalf("DB startup failed: %v", err)
	}

	// 3. DBの起動待ち
	fmt.Println("Waiting for DB to be ready...")
	time.Sleep(5 * time.Second)

	// 4. Migration
	if err := run("docker", "compose", "run", "--rm", "migrate"); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	if err := run("docker", "compose", "up", "-d"); err != nil {
		log.Fatalf("Services startup failed: %v", err)
	}

	fmt.Println("✓ Deployment finished successfully!")
}
