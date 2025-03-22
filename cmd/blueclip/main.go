package main

import (
	"blueclip/pkg/db"
	"blueclip/pkg/service"
	"context"
	"flag"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {

	path := flag.String("path", "~/.cache/blueclip/history.bin", "path to the history file")
	flag.Parse()
	ctx := context.Background()

	switch os.Args[1] {
	case "list":
		client := service.NewClient()
		resp, err := client.List(ctx)
		if err != nil {
			log.Fatalf("Failed to list selections: %v", err)
		}
		defer resp.Body.Close()

		_, err = io.Copy(os.Stdout, resp.Body)
		if err != nil {
			log.Fatalf("Failed to print selection: %v", err)
		}
	case "print":
		client := service.NewClient()
		resp, err := client.Print(ctx, os.Stdin)
		if err != nil {
			log.Fatalf("Failed to print selection: %v", err)
		}
		defer resp.Body.Close()

		_, err = io.Copy(os.Stdout, resp.Body)
		if err != nil {
			log.Fatalf("Failed to print selection: %v", err)
		}
	case "copy":
		client := service.NewClient()
		resp, err := client.Copy(ctx, os.Stdin)
		if err != nil {
			log.Fatalf("Failed to copy selection: %v", err)
		}
		defer resp.Body.Close()

		_, err = io.Copy(os.Stdout, resp.Body)
		if err != nil {
			log.Fatalf("Failed to copy selection: %v", err)
		}
	case "clear":
		client := service.NewClient()
		resp, err := client.Clear(ctx)
		if err != nil {
			log.Fatalf("Failed to clear selections: %v", err)
		}
		defer resp.Body.Close()
	case "watch":
		watchClipboard(*path)
	default:
		log.Fatalf("Unknown command: %s", os.Args[1])
	}

}

func watchClipboard(path string) {
	log.Println("Watching clipboard...")
	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Create a context that we can cancel
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		<-sigChan
		cancel()
		log.Println("Shutting down...")
		<-sigChan
		os.Exit(1)
	}()

	db, err := db.NewFileDB(path)
	if err != nil {
		log.Fatalf("Failed to create db: %v", err)
	}

	service := service.NewService(db)
	err = service.Run(ctx)
	if err != nil {
		log.Fatalf("Failed to run service: %v", err)
	}
}
