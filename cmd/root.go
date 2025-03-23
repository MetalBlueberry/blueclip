/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"blueclip/cmd/client"
	"os"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "blueclip",
	Short: "A command-line tool for managing clipboard selections",
	Long: `Blueclip is a powerful CLI application designed to enhance your clipboard management experience.
It allows users to store, retrieve, and manipulate clipboard selections efficiently.

With Blueclip, you can:
- Save multiple clipboard entries for easy access.
- Integrate with your favourite tools such as fzf, rofi, etc.
- Quickly list your saved selections in order of relevance.
- Clear selections when they are no longer needed.
`,
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(serverCmd)
	client.Register(rootCmd)
}
