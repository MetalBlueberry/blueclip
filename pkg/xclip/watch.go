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
	Silent bool
	// monitorTargets is the list of targets to for changes.
	monitorTargets []ValidTarget

	// targetPriority is the priority of the target to use.
	// In case of multiple targets, the first one in the list will be used.
	targetPriority []ValidTarget

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

func WatchOptionWithMonitorTargets(targets []ValidTarget) WatchOption {
	return func(o *WatchOptions) error {
		o.monitorTargets = targets
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

func WatchOptionWithTargetPriority(priority []ValidTarget) WatchOption {
	return func(o *WatchOptions) error {
		o.targetPriority = priority
		return nil
	}
}

func desiredTarget(targets []ValidTarget, priority []ValidTarget) ValidTarget {
	for _, target := range priority {
		if slices.Contains(targets, target) {
			return target
		}
	}
	return ValidTargetUnknown
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
		initialCtx, cancel := context.WithTimeout(ctx, opts.frequency)

		previous := bytes.NewBuffer([]byte{})
		current := bytes.NewBuffer([]byte{})
		commonPasteOpts := []PasteOption{
			PasteOptionWithSelection(opts.clip),
		}

		monitorTarget, _ := findValidTarget(initialCtx, opts.monitorTargets, opts)
		if monitorTarget != ValidTargetUnknown {
			err := x.Paste(initialCtx, previous, append(commonPasteOpts, PasteOptionWithTarget(monitorTarget))...)
			if err != nil {
				log.Fatalf("Failed to read inital clipboard: %v", err)
			}
		}
		cancel()

		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(opts.frequency):
				func() {
					ctx, cancel := context.WithTimeout(ctx, opts.frequency)
					defer cancel()

					monitorTarget, allTargets := findValidTarget(ctx, opts.monitorTargets, opts)
					if monitorTarget == ValidTargetUnknown {
						return
					}

					current.Reset()
					err := x.Paste(ctx, current, append(commonPasteOpts, PasteOptionWithTarget(monitorTarget))...)
					if err != nil {
						log.Printf("Failed to read clipboard: %v", err)
						return
					}
					if bytes.Equal(previous.Bytes(), current.Bytes()) {
						return
					}

					log.Printf("Detected change in clipboard %s", opts.clip)
					previous.Reset()
					current.WriteTo(previous)

					withTarget := desiredTarget(allTargets, opts.targetPriority)
					if withTarget == ValidTargetUnknown {
						log.Printf("No valid target found to read, available targets: %v, valid targets: %v", allTargets, opts.targetPriority)
						return
					}
					buf := bytes.NewBuffer([]byte{})
					err = x.Paste(
						ctx,
						buf,
						append(commonPasteOpts, PasteOptionWithTarget(withTarget))...,
					)
					if err != nil {
						log.Printf("Failed to read clipboard: %v", err)
						return
					}

					select {
					case <-ctx.Done():
						return
					case ch <- NewSelection(buf.Bytes(), withTarget):
					}
				}()
			}
		}
	}()
	return ch
}

func findValidTarget(ctx context.Context, targets []ValidTarget, opts *WatchOptions) (ValidTarget, []ValidTarget) {
	currentTargets, err := Targets(
		ctx,
		TargetsOptionWithSelection(opts.clip),
	)
	if err != nil {
		return ValidTargetUnknown, nil
	}
	for _, target := range opts.monitorTargets {
		if slices.Contains(currentTargets, target) {
			return target, currentTargets
		}
	}
	return ValidTargetUnknown, currentTargets
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
	ValidTargetUnknown                          ValidTarget = ""
	ValidTargetTIMESTAMP                        ValidTarget = "TIMESTAMP"
	ValidTargetTARGETS                          ValidTarget = "TARGETS"
	ValidTargetMULTIPLE                         ValidTarget = "MULTIPLE"
	ValidTargetUTF8_STRING                      ValidTarget = "UTF8_STRING"
	ValidTargetCOMPOUND_TEXT                    ValidTarget = "COMPOUND_TEXT"
	ValidTargetTEXT                             ValidTarget = "TEXT"
	ValidTargetSTRING                           ValidTarget = "STRING"
	ValidTargetTextPlainUTF8                    ValidTarget = "text/plain;charset=utf-8"
	ValidTargetTextPlain                        ValidTarget = "text/plain"
	ValidTargetSAVE_TARGETS                     ValidTarget = "SAVE_TARGETS"
	ValidTargetxSpecialGnomeCopiedFiles         ValidTarget = "x-special/gnome-copied-files"
	ValidTargetApplicationVndPortalFiletransfer ValidTarget = "application/vnd.portal.filetransfer"
	ValidTargetApplicationVndPortalFiles        ValidTarget = "application/vnd.portal.files"
	ValidTargetTextUriList                      ValidTarget = "text/uri-list"
	ValidTargetImagePng                         ValidTarget = "image/png"
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

func Targets(ctx context.Context, opt ...TargetsOption) ([]ValidTarget, error) {
	opts := &TargetsOptions{
		Silent:   true,
		ExecerFn: realRunner,
	}
	for _, opt := range opt {
		opt(opts)
	}

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

	cmd := exec.CommandContext(ctx, "xclip")
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
