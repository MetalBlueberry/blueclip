package xclip

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os/exec"
	"slices"
	"strings"
	"time"
)

type WatchOptions struct {
	Silent    bool
	targets   []ValidTarget
	clip      ClipboardSelection
	frequency time.Duration
}

type WatchOption func(*WatchOptions) error

func WatchOptionWithClipboardSelection(clip ClipboardSelection) WatchOption {
	return func(o *WatchOptions) error {
		o.clip = clip
		return nil
	}
}

func WatchOptionWithTargets(targets []ValidTarget) WatchOption {
	return func(o *WatchOptions) error {
		o.targets = targets
		return nil
	}
}

func WatchOptionWithFrequency(frequency time.Duration) WatchOption {
	return func(o *WatchOptions) error {
		if o.frequency == 0 {
			return fmt.Errorf("frequency must be greater than 0")
		}
		o.frequency = frequency
		return nil
	}
}

func (x *XClip) Watch(ctx context.Context, opt ...WatchOption) <-chan Selection {
	opts := &WatchOptions{
		Silent:    true,
		frequency: time.Second,
	}
	for _, opt := range opt {
		opt(opts)
	}

	ch := make(chan Selection)

	go func() {
		defer close(ch)

		previous := bytes.NewBuffer([]byte{})
		current := bytes.NewBuffer([]byte{})
		commonPasteOpts := []PasteOption{
			PasteOptionWithSelection(opts.clip),
		}

		validTarget := findValidTarget(opts.targets, opts)
		if validTarget != ValidTargetUnknown {
			err := x.Paste(previous, append(commonPasteOpts, PasteOptionWithTarget(validTarget))...)
			if err != nil {
				log.Fatalf("Failed to read clipboard: %v", err)
			}
		}

		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(opts.frequency):
				validTarget := findValidTarget(opts.targets, opts)
				if validTarget == ValidTargetUnknown {
					continue
				}

				current.Reset()
				err := x.Paste(
					current,
					append(commonPasteOpts, PasteOptionWithTarget(validTarget))...,
				)
				if err != nil {
					log.Fatalf("Failed to read clipboard: %v", err)
				}
				if !bytes.Equal(previous.Bytes(), current.Bytes()) {
					selectionBytes := current.Bytes()
					selectionCopy := make([]byte, len(selectionBytes))
					copy(selectionCopy, selectionBytes)
					ch <- NewSelection(selectionCopy, validTarget)
					previous.Reset()
					current.WriteTo(previous)
				}
			}
		}
	}()
	return ch
}

func findValidTarget(targets []ValidTarget, opts *WatchOptions) ValidTarget {
	currentTargets, err := Targets(
		TargetsOptionWithSelection(opts.clip),
	)
	if err != nil {
		return ValidTargetUnknown
	}
	for _, target := range opts.targets {
		if slices.Contains(currentTargets, target) {
			return target
		}
	}
	return ValidTargetUnknown
}

type ReadResult struct {
	Content string
	Target  []string
}

type TargetsOption func(*TargetsOptions)

type TargetsOptions struct {
	Display   string             // -d, -display
	Selection ClipboardSelection // -selection
	Silent    bool               // -silent
	Quiet     bool               // -quiet
	Verbose   bool               // -verbose

	ExecerFn func(*exec.Cmd) error
}

type ValidTarget string

const (
	ValidTargetUnknown       ValidTarget = ""
	ValidTargetTIMESTAMP     ValidTarget = "TIMESTAMP"
	ValidTargetTARGETS       ValidTarget = "TARGETS"
	ValidTargetMULTIPLE      ValidTarget = "MULTIPLE"
	ValidTargetUTF8_STRING   ValidTarget = "UTF8_STRING"
	ValidTargetCOMPOUND_TEXT ValidTarget = "COMPOUND_TEXT"
	ValidTargetTEXT          ValidTarget = "TEXT"
	ValidTargetSTRING        ValidTarget = "STRING"
	ValidTargetTextPlainUTF8 ValidTarget = "text/plain;charset=utf-8"
	ValidTargetTextPlain     ValidTarget = "text/plain"
)

func TargetsOptionWithSelection(selection ClipboardSelection) TargetsOption {
	return func(o *TargetsOptions) {
		o.Selection = selection
	}
}

func TargetsOptionWithExecerFn(execerFn func(*exec.Cmd) error) TargetsOption {
	return func(o *TargetsOptions) {
		o.ExecerFn = execerFn
	}
}

func Targets(opt ...TargetsOption) ([]ValidTarget, error) {
	opts := &TargetsOptions{
		Silent:   true,
		ExecerFn: realRunner,
	}
	for _, opt := range opt {
		opt(opts)
	}

	cmd := exec.Command("xclip")
	args := []string{"-o", "-target", "TARGETS"} // output target mode

	if opts.Selection != "" {
		// Add selection type
		args = append(args, "-selection", string(opts.Selection))
	}

	// Add display if specified
	if opts.Display != "" {
		args = append(args, "-d", opts.Display)
	}

	if opts.Silent {
		args = append(args, "-silent")
	}
	if opts.Quiet {
		args = append(args, "-quiet")
	}
	if opts.Verbose {
		args = append(args, "-verbose")
	}

	cmd.Args = append([]string{"xclip"}, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := opts.ExecerFn(cmd); err != nil {
		return nil, fmt.Errorf("xclip failed: %v, stderr: %s", err, stderr.String())
	}

	availableTargets := strings.Split(stdout.String(), "\n")

	validTargets := make([]ValidTarget, len(availableTargets))
	for i, target := range availableTargets {
		validTargets[i] = ValidTarget(target)
	}

	return validTargets, nil
}
