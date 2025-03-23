package client

import (
	"blueclip/pkg/service"
	"context"
	"io"
	"log"

	"github.com/spf13/cobra"
)

var copyCmd = &cobra.Command{
	Use:   "copy",
	Short: "Copy the clipboard entry to the clipboard",
	Long: `Copy the clipboard entry to the clipboard
You must pipe to the stdin the line of the selection you want to copy.
When integrating with tools such as fzf you can do it in the following way:

Example:
blueclip list | fzf | blueclip copy`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()

		client := service.NewClient(socketPath)
		resp, err := client.Copy(ctx, cmd.InOrStdin())
		if err != nil {
			log.Fatalf("Failed to copy selection: %v", err)
		}
		defer resp.Body.Close()

		_, err = io.Copy(cmd.OutOrStdout(), resp.Body)
		if err != nil {
			log.Fatalf("Failed to copy selection: %v", err)
		}
	},
}
