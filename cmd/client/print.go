package client

import (
	"blueclip/pkg/service"
	"context"
	"io"
	"log"

	"github.com/spf13/cobra"
)

var printCmd = &cobra.Command{
	Use:   "print",
	Short: "Print a single selection",
	Long: `Print a single selection from the clipboard history
Intended to be a preview of the selection
By default it prints the selection "as is", but it can be adjusted with flags.

Example:
blueclip list | fzf --preview-window right:wrap --preview 'echo {} | blueclip print' | blueclip copy`,

	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()

		client := service.NewClient(socketPath)
		resp, err := client.Print(ctx, cmd.InOrStdin())
		if err != nil {
			log.Fatalf("Failed to print selection: %v", err)
		}
		defer resp.Body.Close()

		_, err = io.Copy(cmd.OutOrStdout(), resp.Body)
		if err != nil {
			log.Fatalf("Failed to print selection: %v", err)
		}
	},
}
