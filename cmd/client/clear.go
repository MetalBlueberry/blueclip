package client

import (
	"blueclip/pkg/service"
	"context"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/spf13/cobra"
)

var clearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear the clipboard",

	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()

		client := service.NewClient(socketPath)

		clearType, err := cmd.Flags().GetString("type")
		if err != nil {
			log.Fatalf("Failed to get type flag: %v", err)
		}

		clearAll, err := cmd.Flags().GetBool("all")
		if err != nil {
			log.Fatalf("Failed to get all flag: %v", err)
		}
		if clearAll {
			cmd.SetIn(strings.NewReader(""))
		}

		resp, err := client.Clear(ctx, cmd.InOrStdin(), service.ClearWithType(clearType))
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
	clearCmd.Flags().Bool("all", false, "clear all items, if not specified, it will read from stdin")
}
