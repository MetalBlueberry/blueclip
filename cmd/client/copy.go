package client

import (
	"blueclip/pkg/service"
	"blueclip/pkg/xclip"
	"context"
	"io"
	"log"
	"net/http"

	"github.com/spf13/cobra"
)

var copyCmd = &cobra.Command{
	Use:   "copy",
	Short: "Copy the clipboard entry to the clipboard",
	Long: `Copy the clipboard entry to the clipboard
You must pipe to the stdin the line of the selection you want to copy.
When integrating with tools such as fzf you can do it in the following way:

Example:
blueclip list | fzf | blueclip copy

You can also specify the clipboard selection to copy to:

Example:
blueclip list | fzf | blueclip copy --clipboard-selection primary

or even replicate the copy to multiple clipboard selections

Example:
blueclip list | fzf | blueclip copy -c primary -c clipboard

`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()

		client := service.NewClient(socketPath)
		clipboardSelections, err := cmd.Flags().GetStringArray("clipboard-selection")
		if err != nil {
			log.Fatalf("Failed to get clipboard-selection flag: %v", err)
		}
		for _, s := range clipboardSelections {
			if xclip.ClipboardSelection(s) != xclip.ClipboardSelectionPrimary &&
				xclip.ClipboardSelection(s) != xclip.ClipboardSelectionSecondary &&
				xclip.ClipboardSelection(s) != xclip.ClipboardSelectionClipboard {
				log.Fatalf("Invalid clipboard selection: %s, valid values are: %v", s, []string{
					string(xclip.ClipboardSelectionPrimary),
					string(xclip.ClipboardSelectionSecondary),
					string(xclip.ClipboardSelectionClipboard),
				})
			}
		}

		resp, err := client.Copy(
			ctx,
			cmd.InOrStdin(),
			service.CopyWithClipboardSelection(clipboardSelections),
		)
		if err != nil {
			log.Fatalf("Failed to copy selection: %v", err)
		}
		defer resp.Body.Close()

		_, err = io.Copy(cmd.OutOrStdout(), resp.Body)
		if err != nil {
			log.Fatalf("Failed to copy selection: %v", err)
		}

		if resp.StatusCode != http.StatusOK {
			log.Fatalf("Failed to copy selection: %v", resp.Status)
		}
	},
}

func init() {
	copyCmd.Flags().StringArrayP("clipboard-selection", "c", []string{"clipboard"}, "x11 clipboard selection to copy to [primary, secondary, clipboard]")
}
