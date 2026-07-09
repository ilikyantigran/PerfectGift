package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ilikyantigran/PerfectGift/services/backend/api-gateway/internal/app"
)

func main() {
	path := os.Getenv("CONFIG_PATH")
	if path == "" {
		path = "./configs/values_local.yaml"
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	a, err := app.NewApp(path)
	if err != nil {
		log.Fatalf("init app: %v", err)
	}

	if err := a.Run(ctx); err != nil {
		log.Fatalf("run: %v", err)
	}
}
