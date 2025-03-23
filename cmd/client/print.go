package client

import (
	"blueclip/pkg/service"
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"

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

		unindent, err := cmd.Flags().GetBool("unindent")
		if err != nil {
			log.Fatalf("Failed to get unindent flag: %v", err)
		}

		width, err := cmd.Flags().GetInt("width")
		if err != nil {
			log.Fatalf("Failed to get width flag: %v", err)
		}

		height, err := cmd.Flags().GetInt("height")
		if err != nil {
			log.Fatalf("Failed to get height flag: %v", err)
		}

		resp, err := client.Print(
			ctx,
			cmd.InOrStdin(),
			service.PrintWithUnindent(unindent),
			service.PrintWithDimensions(width, height),
		)
		if err != nil {
			log.Fatalf("Failed to print selection: %v", err)
		}
		defer resp.Body.Close()

		_, err = io.Copy(cmd.OutOrStdout(), resp.Body)
		if err != nil {
			log.Fatalf("Failed to print selection: %v", err)
		}

		if resp.StatusCode != http.StatusOK {
			log.Fatalf("Failed to print selection: %v", resp.Status)
		}
	},
}

func init() {
	printCmd.Flags().BoolP("unindent", "u", false, "Unindent the selection")

	widthInt := 0
	width, ok := os.LookupEnv("FZF_PREVIEW_COLUMNS")
	if ok {
		widthInt, _ = strconv.Atoi(width)
		widthInt--
	}

	heightInt := 0
	height, ok := os.LookupEnv("FZF_PREVIEW_LINES")
	if ok {
		heightInt, _ = strconv.Atoi(height)
		heightInt--
	}

	printCmd.Flags().Int("width", widthInt, "Width of the selection")
	printCmd.Flags().Int("height", heightInt, "Height of the selection")
}
