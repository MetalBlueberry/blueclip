package xclip

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
)

type PasteOptions struct {
	loops        int                // -l, -loops
	display      string             // -d, -display
	selection    ClipboardSelection // -selection
	noUTF8       bool               // -noutf8
	target       string             // -target
	removeLastNL bool               // -rmlastnl
	silent       bool               // -silent
	quiet        bool               // -quiet
	verbose      bool               // -verbose
}

type PasteOption func(*PasteOptions) error

type PasteResult struct {
	Content string
	Target  string
}

func PasteOptionWithTarget(target ValidTarget) PasteOption {
	return func(o *PasteOptions) error {
		o.target = string(target)
		return nil
	}
}

func PasteOptionWithSelection(selection ClipboardSelection) PasteOption {
	return func(o *PasteOptions) error {
		o.selection = selection
		return nil
	}
}

// Paste retrieves text from clipboard
func (x *XClip) Paste(ctx context.Context, out io.Writer, opt ...PasteOption) error {
	opts := &PasteOptions{
		silent: true,
	}
	for _, opt := range opt {
		opt(opts)
	}

	args := []string{"-o"} // output mode

	if opts.selection != ClipboardSelectionUndefined {
		args = append(args, "-selection", string(opts.selection))
	}

	// Add display if specified
	if opts.display != "" {
		args = append(args, "-d", opts.display)
	}

	// Add loops if specified
	if opts.loops > 0 {
		args = append(args, "-l", fmt.Sprintf("%d", opts.loops))
	}

	// Add target if specified
	if opts.target != "" {
		args = append(args, "-target", opts.target)
	}

	// Add flags
	if opts.noUTF8 {
		args = append(args, "-noutf8")
	}
	if opts.removeLastNL {
		args = append(args, "-rmlastnl")
	}
	if opts.silent {
		args = append(args, "-silent")
	}
	if opts.quiet {
		args = append(args, "-quiet")
	}
	if opts.verbose {
		args = append(args, "-verbose")
	}

	var stderr bytes.Buffer

	cmd := exec.CommandContext(ctx, "xclip")
	cmd.Args = append([]string{"xclip"}, args...)
	cmd.Stdout = out
	cmd.Stderr = &stderr

	if err := x.RunFn(cmd); err != nil {
		return fmt.Errorf("xclip failed: %v, stderr: %s", err, stderr.String())
	}

	return nil
}
