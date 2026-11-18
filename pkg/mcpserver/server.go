// Package mcpserver exposes the clipboard history to AI agents over the Model
// Context Protocol. It is a thin bridge to the JSON API of the blueclip server.
package mcpserver

import (
	"blueclip/pkg/service"
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const instructions = `blueclip is the user's clipboard history on X11 (Linux).

Use these tools whenever the user asks what is in their clipboard, about something they copied, or to copy something for them. Prefer them over xclip, xsel or wl-copy: copies made with those tools are not recorded in the history.

Entries come from explicit copies (source "clipboard") and from text the user highlighted (source "primary"). An entry's time is when it was last copied or selected; entries without a time were saved before times were recorded and are the oldest. Entries the user picked again from the history are kind "important" and are kept longer than "ephemeral" ones.

list_entries returns entries newest first with their content truncated; use get_entry for the full content. Prefer filtering with query, regex, since and source over paging through everything.

set_clipboard and copy_entry replace what the user pastes next. delete_entries removes entries from the history permanently.`

type ListInput struct {
	Query    string `json:"query,omitempty" jsonschema:"words that must all appear in the content, ignoring case"`
	Regex    string `json:"regex,omitempty" jsonschema:"RE2 regular expression the content must match"`
	Since    string `json:"since,omitempty" jsonschema:"only entries copied at or after this time: RFC 3339, a local date (2026-11-18), a local date and time (2026-11-18T09:30) or a duration ago (30m, 2h, 3d). Excludes entries without a time"`
	Until    string `json:"until,omitempty" jsonschema:"only entries copied at or before this time, same formats as since"`
	Source   string `json:"source,omitempty" jsonschema:"clipboard for explicit copies, primary for highlighted text"`
	Kind     string `json:"kind,omitempty" jsonschema:"important or ephemeral"`
	Limit    int    `json:"limit,omitempty" jsonschema:"maximum number of entries to return, default 20, at most 500"`
	Offset   int    `json:"offset,omitempty" jsonschema:"number of matching entries to skip, for paging"`
	MaxChars int    `json:"max_chars,omitempty" jsonschema:"truncate each content to this many characters, default 300, -1 for no limit"`
}

type IDInput struct {
	ID uint64 `json:"id" jsonschema:"entry ID from list_entries"`
}

type CopyInput struct {
	ID         uint64   `json:"id" jsonschema:"entry ID from list_entries"`
	Selections []string `json:"selections,omitempty" jsonschema:"X11 selections to copy to: clipboard, primary or secondary. Default clipboard"`
}

type SetInput struct {
	Text       string   `json:"text" jsonschema:"text to put in the clipboard, exactly as it should be pasted"`
	Selections []string `json:"selections,omitempty" jsonschema:"X11 selections to copy to: clipboard, primary or secondary. Default clipboard"`
}

type DeleteInput struct {
	IDs []uint64 `json:"ids" jsonschema:"entry IDs to delete"`
}

// New returns an MCP server whose tools call the blueclip server through client.
func New(client *service.Client, version string) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "blueclip", Version: version}, &mcp.ServerOptions{
		Instructions: instructions,
	})

	destructive := true
	notDestructive := false

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_entries",
		Description: "List clipboard history entries, newest first, with optional filters. Content is truncated, see max_chars and get_entry.",
		Annotations: &mcp.ToolAnnotations{Title: "List clipboard history", ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, service.ListEntriesResponse, error) {
		out, err := client.ListEntries(ctx, service.ListQuery{
			Query:    in.Query,
			Regex:    in.Regex,
			Since:    in.Since,
			Until:    in.Until,
			Source:   in.Source,
			Kind:     in.Kind,
			Limit:    in.Limit,
			Offset:   in.Offset,
			MaxChars: in.MaxChars,
		})
		return nil, out, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_entry",
		Description: "Get one clipboard history entry with its full content.",
		Annotations: &mcp.ToolAnnotations{Title: "Get clipboard entry", ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in IDInput) (*mcp.CallToolResult, service.APIEntry, error) {
		out, err := client.GetEntry(ctx, in.ID)
		return nil, out, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "set_clipboard",
		Description: "Put new text in the user's clipboard, ready to paste, and record it in the history. Returns the new entry.",
		Annotations: &mcp.ToolAnnotations{Title: "Set clipboard", DestructiveHint: &notDestructive},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in SetInput) (*mcp.CallToolResult, service.APIEntry, error) {
		out, err := client.SetClipboard(ctx, in.Text, in.Selections)
		return nil, out, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "copy_entry",
		Description: "Copy an existing history entry back into the user's clipboard. The entry becomes important and current, like picking it in the history.",
		Annotations: &mcp.ToolAnnotations{Title: "Copy clipboard entry", DestructiveHint: &notDestructive},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in CopyInput) (*mcp.CallToolResult, service.APIEntry, error) {
		out, err := client.CopyEntry(ctx, in.ID, in.Selections)
		return nil, out, err
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "delete_entries",
		Description: "Permanently delete entries from the clipboard history by ID. This cannot be undone, so unless the user named the exact entries, show them what will be deleted first.",
		Annotations: &mcp.ToolAnnotations{Title: "Delete clipboard entries", DestructiveHint: &destructive, IdempotentHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in DeleteInput) (*mcp.CallToolResult, service.DeleteEntriesResponse, error) {
		out, err := client.DeleteEntries(ctx, in.IDs)
		return nil, out, err
	})

	return server
}
