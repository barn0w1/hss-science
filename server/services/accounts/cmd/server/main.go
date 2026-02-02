package main

import (
	"log/slog"
	"os"

	"github.com/barn0w1/hss-science/server/services/accounts/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		slog.Error("Server exited", "error", err)
		os.Exit(1)
	}
}
