package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type Application struct {
	configPath string
	debug      bool
}

func main() {
	app := &Application{}
	
	flag.StringVar(&app.configPath, "config", "config/dev.yaml", "Configuration file path")
	flag.BoolVar(&app.debug, "debug", false, "Enable debug mode")
	flag.Parse()

	if err := app.Run(); err != nil {
		log.Fatalf("Application failed: %v", err)
	}
}

func (app *Application) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fmt.Printf("Starting sitemap-go server...\n")
	fmt.Printf("Config: %s\n", app.configPath)
	fmt.Printf("Debug: %t\n", app.debug)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nShutdown signal received...")
		cancel()
	}()

	fmt.Println("Server started successfully")
	fmt.Println("Press Ctrl+C to stop")
	
	<-ctx.Done()
	fmt.Println("Shutting down gracefully...")
	
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	
	<-shutdownCtx.Done()
	fmt.Println("Server stopped")
	
	return nil
}