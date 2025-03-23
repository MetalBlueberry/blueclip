package cmd

import (
	"blueclip/pkg/db"
	"blueclip/pkg/service"
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the server",
	Long:  `Start the server`,
	Run: func(cmd *cobra.Command, args []string) {
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

		path, err := cmd.Flags().GetString("history")
		if err != nil {
			log.Fatalf("Failed to get path to the history file: %v", err)
		}

		db, err := db.NewFileDB(path)
		if err != nil {
			log.Fatalf("Failed to create db: %v", err)
		}

		service := service.NewService(db)
		err = service.Run(ctx)
		if err != nil {
			log.Fatalf("Failed to run service: %v", err)
		}
	},
}

func init() {
	serverCmd.Flags().StringP("history", "p", "~/.cache/blueclip/history.bin", "path to the history file")
	serverCmd.Flags().StringP("config", "c", "~/.config/blueclip.yaml", "path to the config file")
}
