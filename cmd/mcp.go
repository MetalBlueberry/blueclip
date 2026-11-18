package cmd

import (
	"blueclip/pkg/mcpserver"
	"blueclip/pkg/service"
	"context"
	"log"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Serve the clipboard history to AI agents over MCP",
	Long: `Serve the clipboard history to AI agents using the Model Context Protocol over stdio.

It is started by the agent, not by hand, and talks to the running blueclip server.

Example, to make it available to Claude Code in every project:
claude mcp add --scope user blueclip -- blueclip mcp`,
	Run: func(cmd *cobra.Command, args []string) {
		socketPath, err := cmd.Flags().GetString("socket")
		if err != nil {
			log.Fatalf("Failed to get socket path: %v", err)
		}

		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		server := mcpserver.New(service.NewClient(socketPath), version())
		// stdout carries the protocol, logs go to stderr
		log.SetOutput(os.Stderr)
		if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil && ctx.Err() == nil {
			log.Fatalf("MCP server failed: %v", err)
		}
	},
}

func version() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		return info.Main.Version
	}
	return "devel"
}

func init() {
	mcpCmd.Flags().StringP("socket", "s", service.DefaultSocketPath(), "path to the unix socket")
}
