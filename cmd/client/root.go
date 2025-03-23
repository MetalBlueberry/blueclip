package client

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var socketPath string

var rootCmd = &cobra.Command{
	Use:   "client",
	Short: "Interact with the blueclip server",
	Long: `Interact with the blueclip server

The communication happens over a unix socket using http protocol`,
}

func Register(base *cobra.Command) {
	base.AddCommand(rootCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(printCmd)
	rootCmd.AddCommand(copyCmd)
	rootCmd.AddCommand(clearCmd)
}

func init() {
	socket := filepath.Join(os.TempDir(), "blueclip", "blueclip.sock")
	rootCmd.PersistentFlags().StringVarP(&socketPath, "socket", "s", socket, "path to the unix socket")
}
