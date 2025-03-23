package client

import (
	"blueclip/pkg/service"
	"context"
	"io"
	"log"
	"net/http"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List the clipboard history as single lines",
	Long: `List the clipboard history as single lines
Intended to be piped to other commands such as fzf, rofi, etc.`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()

		client := service.NewClient(socketPath)
		resp, err := client.List(ctx)
		if err != nil {
			log.Fatalf("Failed to list selections: %v", err)
		}
		defer resp.Body.Close()

		_, err = io.Copy(cmd.OutOrStdout(), resp.Body)
		if err != nil {
			log.Fatalf("Failed to print selection: %v", err)
		}

		if resp.StatusCode != http.StatusOK {
			log.Fatalf("Failed to list selections: %v", resp.Status)
		}
	},
}
