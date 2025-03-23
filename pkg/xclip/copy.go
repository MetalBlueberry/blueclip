package xclip

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
)

type ClipboardSelection string

const (
	ClipboardSelectionUndefined ClipboardSelection = ""
	ClipboardSelectionPrimary   ClipboardSelection = "primary"
	ClipboardSelectionSecondary ClipboardSelection = "secondary"
	ClipboardSelectionClipboard ClipboardSelection = "clipboard"
)

type CopyOptions struct {
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

type CopyOption func(*CopyOptions) error

func CopyOptionSelection(selection ClipboardSelection) CopyOption {
	return func(opts *CopyOptions) error {
		switch selection {
		case ClipboardSelectionPrimary,
			ClipboardSelectionSecondary,
			ClipboardSelectionClipboard:
			opts.selection = selection
		default:
			return fmt.Errorf("invalid clipboard selection: %s, valid values are: %v", selection, []ClipboardSelection{
				ClipboardSelectionPrimary,
				ClipboardSelectionSecondary,
				ClipboardSelectionClipboard,
			})
		}
		return nil
	}
}
func CopyOptionWithTarget(priority ValidTarget) CopyOption {
	return func(opts *CopyOptions) error {
		opts.target = string(priority)
		return nil
	}
}

// Copy copies text to clipboard
func (x *XClip) Copy(data io.Reader, opt ...CopyOption) error {
	opts := &CopyOptions{
		silent: true,
	}
	for _, opt := range opt {
		opt(opts)
	}

	cmd := exec.Command("xclip")
	args := []string{"-i"}

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

	cmd.Args = append([]string{"xclip"}, args...)
	cmd.Stdin = data

	// Create a pipe for stderr instead of a buffer
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %v", err)
	}

	// Start reading stderr asynchronously
	errChan := make(chan string, 1)
	go func() {
		var stderr bytes.Buffer
		io.Copy(&stderr, stderrPipe)
		errChan <- stderr.String()
	}()

	if err := x.RunFn(cmd); err != nil {
		// Get the stderr content if available
		select {
		case stderr := <-errChan:
			return fmt.Errorf("xclip failed: %v, stderr: %s", err, stderr)
		default:
			return fmt.Errorf("xclip failed: %v", err)
		}
	}

	return nil
}
