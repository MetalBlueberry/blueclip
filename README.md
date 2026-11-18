# blueclip

my implementation of a clipboard manager for i3

I got inspiration from greenclip, but got disappointed due to some small problems.

- Unable to print selection "as is" so I could display a preview window
- Clipboard full of junk and no easy way to maintain it. like single character copies or meaningless words

So why not, let's implement it myself, how hard it can be?

I wanted something that

- Integrates with fzf
- Monitors clipboard and primary selection
- Allows me to preview multiple lines

So far I have

- [x] Integrates with fzf
- [x] Monitos clipboard, primary and secondary
- [x] supports copy targets UTF8_STRING and x-special/gnome-copied-files (text only, images are not stored)
- [x] has a very nice cli tool to interact with it
- [x] keeps my importan selections nearby

## Selection model

Instead of keeping a simple stack of selections, I've model it with 3 categories.

- Ephemeral selections
- Important selections
- Last selection

Last selection is whatever you just selected, Easy!. It will be always the first thing you see when you list your selections.

Ephemeral selections are everything you select by default. It has a capacity that if reached, it will drop old selections

Important selections are anything that you ever copied, The biggest advantage is that it has a different limit from Ephemeral selections, so it is likely to stay around for much longer.

When you list, you always get Last selection, but then you get a mix of Important/Ephemeral interlocked. Something like

```txt
Last Selection
Important
Ephemeral
Important
Ephemeral
Important
Ephemeral
Important
Ephemeral
```

Reasoning is that you may want to see what is in your current selection, that is why last selection is first thing. Then you are equally likely to be looking for an old important selection or a new recent selection. So both are interlocked.

In any case, you can always filter with fzf. but I hope this will make it quicker for the cases where I just scroll over it. (At least for myself)

## AI agents (MCP)

`blueclip mcp` serves the clipboard history to AI agents using the Model Context Protocol, so you can ask things like "is there anything relevant in my clipboard?", "put that in my clipboard" or "delete the entries about today's changes". It runs over stdio, is started by the agent, and talks to the running blueclip server through the same unix socket as the CLI.

To make it available to Claude Code in every project:

```sh
claude mcp add --scope user blueclip -- blueclip mcp
```

The tools are:

- `list_entries`: entries newest first, filtered by words, regex, time (`since`/`until` as a date, a time or a duration like `2h`), source (`clipboard` or `primary`) and kind, with content truncated
- `get_entry`: one entry with its full content
- `set_clipboard`: put new text in the clipboard
- `copy_entry`: copy an existing entry back to the clipboard
- `delete_entries`: delete entries permanently

Every entry has an ID, the time it was last copied or selected, and the selection it came from. Entries saved by older versions get an ID when the history loads, but no time.

The same operations are available as a JSON API under `/api/` on the socket, see `pkg/service/api.go`.
