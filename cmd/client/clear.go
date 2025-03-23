package client

import (
	"blueclip/pkg/service"
	"context"
	"io"
	"log"
	"net/http"

	"github.com/spf13/cobra"
)

var clearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear the clipboard",

	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()

		client := service.NewClient(socketPath)
		resp, err := client.Clear(ctx, cmd.InOrStdin())
		if err != nil {
			log.Fatalf("Failed to clear clipboard: %v", err)
		}
		defer resp.Body.Close()

		_, err = io.Copy(cmd.OutOrStdout(), resp.Body)
		if err != nil {
			log.Fatalf("Failed to clear clipboard: %v", err)
		}

		if resp.StatusCode != http.StatusOK {
			log.Fatalf("Failed to clear clipboard: %v", resp.Status)
		}
	},
}

func init() {
	clearCmd.Flags().String("type", "all", "type of items to clear, [all, ephemeral, important]")
}
