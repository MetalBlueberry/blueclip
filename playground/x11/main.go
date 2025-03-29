package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
)

const (
	timestampAtomName  = "TIMESTAMP"
	clipboardAtomName  = "CLIPBOARD"
	primaryAtomName    = "PRIMARY"
	targetsAtomName    = "TARGETS"
	utf8StringAtomName = "UTF8_STRING"
	textPlainAtomName  = "text/plain"
	stringAtomName     = "STRING"
	imagePngAtomName   = "image/png"
	imageJpegAtomName  = "image/jpeg"
	imageBmpAtomName   = "image/bmp"
	incrAtomName       = "INCR"
)

// AtomInfo holds information about well-known atoms
type AtomInfo struct {
	timestamp  xproto.Atom
	clipboard  xproto.Atom
	primary    xproto.Atom
	targets    xproto.Atom
	utf8String xproto.Atom
	textPlain  xproto.Atom
	string     xproto.Atom
	imagePng   xproto.Atom
	imageJpeg  xproto.Atom
	imageBmp   xproto.Atom
	incr       xproto.Atom
}

func getAtom(conn *xgb.Conn, name string) xproto.Atom {
	reply, err := xproto.InternAtom(conn, false, uint16(len(name)), name).Reply()
	if err != nil {
		log.Fatalf("Failed to intern atom %s: %v", name, err)
	}
	return reply.Atom
}

func createWindow(conn *xgb.Conn) xproto.Window {
	screen := xproto.Setup(conn).DefaultScreen(conn)
	wid, _ := xproto.NewWindowId(conn)

	log.Print(wid, screen.Root)

	err := xproto.CreateWindowChecked(conn, screen.RootDepth, wid, screen.Root,
		0, 0, 1, 1, 0,
		xproto.WindowClassCopyFromParent,
		0,
		xproto.CwEventMask,
		[]uint32{xproto.EventMaskPropertyChange | xproto.EventMaskStructureNotify},
	).Check()
	if err != nil {
		log.Print("error creating window")
		log.Fatal(err)
	}

	return wid
}

func initAtoms(conn *xgb.Conn) AtomInfo {
	return AtomInfo{
		timestamp:  getAtom(conn, timestampAtomName),
		clipboard:  getAtom(conn, clipboardAtomName),
		primary:    getAtom(conn, primaryAtomName),
		targets:    getAtom(conn, targetsAtomName),
		utf8String: getAtom(conn, utf8StringAtomName),
		textPlain:  getAtom(conn, textPlainAtomName),
		string:     getAtom(conn, stringAtomName),
		imagePng:   getAtom(conn, imagePngAtomName),
		imageJpeg:  getAtom(conn, imageJpegAtomName),
		imageBmp:   getAtom(conn, imageBmpAtomName),
		incr:       getAtom(conn, incrAtomName),
	}
}

// getOwner fetches the name of the window that owns the clipboard
func getOwner(conn *xgb.Conn, clipboard xproto.Atom) xproto.Window {
	reply, err := xproto.GetSelectionOwner(conn, clipboard).Reply()
	if err != nil || reply.Owner == 0 {
		return 0
	}
	return reply.Owner
}

func getOwnerName(conn *xgb.Conn, owner xproto.Window) string {
	nameReply, err := xproto.GetProperty(conn, false, owner, xproto.AtomWmName, xproto.AtomString, 0, 1024).Reply()
	if err != nil || nameReply.ValueLen == 0 {
		return ""
	}
	return string(nameReply.Value)
}

// isIncrementalTransfer checks if the transfer is incremental
func isIncrementalTransfer(conn *xgb.Conn, win xproto.Window, prop xproto.Atom, incr xproto.Atom) bool {
	reply, err := xproto.GetProperty(conn, false, win, prop, xproto.GetPropertyTypeAny, 0, 0).Reply()
	if err != nil {
		return false
	}

	return reply.Type == incr
}

// getSupportedMimes gets the list of supported mime types from the clipboard
func getSupportedMimes(conn *xgb.Conn, win xproto.Window, selection xproto.Atom, targets xproto.Atom) []xproto.Atom {
	// Request the targets

	//xConvertSelection
	//display win
	//clipboard selection
	//targetMime
	//selectionTarget
	//ownWindow  conn?
	//currentTime
	log.Print(win, selection, targets, xproto.TimeCurrentTime)
	xproto.ConvertSelection(conn, win, selection, targets, 0, xproto.TimeCurrentTime).Check()

	// Wait for the SelectionNotify event
	for {
		ev, err := conn.WaitForEvent()
		if err != nil {
			log.Printf("Event error: %v", err)
			continue
		}
		log.Printf("received event: %v", ev)

		if selNotify, ok := ev.(xproto.SelectionNotifyEvent); ok {
			if selNotify.Property == xproto.AtomNone {
				return nil
			}

			reply, err := xproto.GetProperty(conn, false, win, selNotify.Property, xproto.AtomAtom, 0, (1<<32)-1).Reply()
			if err != nil {
				log.Printf("Failed to get targets: %v", err)
				return nil
			}

			// Convert the []byte to []xproto.Atom
			formats := make([]xproto.Atom, len(reply.Value)/4)
			for i := 0; i < len(formats); i++ {
				formats[i] = xproto.Atom(xgb.Get32(reply.Value[i*4:]))
			}

			// Delete the property to let the owner know we've read it
			xproto.DeleteProperty(conn, win, selNotify.Property)

			return formats
		}
	}
}

// getPriorityFormat chooses the best available format from the supported formats
func getPriorityFormat(supportedFormats []xproto.Atom, atoms AtomInfo) (xproto.Atom, bool) {
	// Priority order for formats
	priorities := []xproto.Atom{
		atoms.utf8String,
		atoms.textPlain,
		atoms.string,
		atoms.imagePng,
		atoms.imageJpeg,
		atoms.imageBmp,
	}

	for _, prio := range priorities {
		for _, format := range supportedFormats {
			if format == prio {
				return format, true
			}
		}
	}

	return 0, false
}

func readClipboard(useImages bool, usePrimary bool) {
	// Try to get an existing connection, or create a new one
	conn, err := xgb.NewConn()
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	atoms := initAtoms(conn)
	win := createWindow(conn)
	defer xproto.DestroyWindow(conn, win)

	// Choose selection based on parameter
	selection := atoms.clipboard
	if usePrimary {
		selection = atoms.primary
	}

	// Get owner window name
	owner := getOwner(conn, selection)
	ownerName := getOwnerName(conn, owner)
	fmt.Println("Clipboard owner:", ownerName)

	// First get the supported target formats
	supportedFormats := getSupportedMimes(conn, win, selection, atoms.targets)

	if len(supportedFormats) == 0 {
		fmt.Println("No formats available in clipboard")
		return
	}

	fmt.Println("Available formats:")
	for _, f := range supportedFormats {
		name, err := xproto.GetAtomName(conn, f).Reply()
		if err == nil {
			fmt.Printf("  - %s\n", name.Name)
		}
	}

	// Choose the format to request based on priority
	targetFormat, found := getPriorityFormat(supportedFormats, atoms)
	if !found {
		fmt.Println("No supported format found in clipboard")
		return
	}

	// Get the atom name for logging
	formatName, _ := xproto.GetAtomName(conn, targetFormat).Reply()
	fmt.Printf("Using format: %s\n", formatName.Name)

	// Request the content with the chosen format
	xproto.ConvertSelection(conn, win, selection, targetFormat, targetFormat, xproto.TimeCurrentTime)

	// Wait for the data
	for {
		ev, err := conn.WaitForEvent()
		if err != nil {
			log.Printf("Event error: %v", err)
			continue
		}

		if selNotify, ok := ev.(xproto.SelectionNotifyEvent); ok {
			if selNotify.Property == xproto.AtomNone {
				fmt.Println("Selection conversion failed")
				return
			}

			// Check if it's an incremental transfer
			if isIncrementalTransfer(conn, win, selNotify.Property, atoms.incr) {
				fmt.Println("Incremental transfer not supported")
				return
			}

			// Get the property data
			reply, err := xproto.GetProperty(conn, false, win, selNotify.Property, selNotify.Target, 0, (1<<32)-1).Reply()
			if err != nil {
				log.Fatalf("Failed to get property: %v", err)
			}

			// Handle based on format
			switch targetFormat {
			case atoms.utf8String, atoms.string, atoms.textPlain:
				fmt.Println("Text content:", string(reply.Value))
			case atoms.imagePng:
				fmt.Printf("PNG image data: %d bytes\n", len(reply.Value))
				// Here you would save or process the PNG data
			case atoms.imageJpeg:
				fmt.Printf("JPEG image data: %d bytes\n", len(reply.Value))
				// Here you would save or process the JPEG data
			case atoms.imageBmp:
				fmt.Printf("BMP image data: %d bytes\n", len(reply.Value))
				// Here you would save or process the BMP data
			default:
				fmt.Printf("Unknown format data: %d bytes\n", len(reply.Value))
			}

			// Delete the property to let the owner know we've read it
			xproto.DeleteProperty(conn, win, selNotify.Property)
			return
		}
	}
}

func writeClipboard(text string, usePrimary bool) {
	// Try to get an existing connection, or create a new one
	conn, err := xgb.NewConn()
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	atoms := initAtoms(conn)
	win := createWindow(conn)
	defer xproto.DestroyWindow(conn, win)

	// Choose selection based on parameter
	selection := atoms.clipboard
	if usePrimary {
		selection = atoms.primary
	}

	// Take ownership of clipboard
	xproto.SetSelectionOwner(conn, win, selection, xproto.TimeCurrentTime)
	owner, err := xproto.GetSelectionOwner(conn, selection).Reply()
	if err != nil {
		log.Fatalf("Failed to get selection owner: %v", err)
	}
	if owner.Owner != win {
		log.Fatalf("Failed to take clipboard ownership, owner: %d, expected: %d", owner.Owner, win)
	}
	log.Printf("Clipboard ownership taken")

	// Define the supported targets
	targets := []xproto.Atom{atoms.timestamp, atoms.targets, atoms.utf8String, atoms.string, atoms.textPlain}
	targetData := make([]byte, len(targets)*4)
	for i, a := range targets {
		xgb.Put32(targetData[i*4:], uint32(a))
	}

	// Event handling loop
	textBytes := []byte(text)
	done := false
	timestamp := time.Now().Unix()
	for !done {
		ev, err := conn.WaitForEvent()
		if err != nil {
			fmt.Printf("Event error: %v\n", err)
			continue
		}

		switch e := ev.(type) {
		case xproto.SelectionRequestEvent:
			log.Printf("SelectionRequestEvent: %v", e)
			var prop xproto.Atom = e.Property
			if prop == xproto.AtomNone {
				prop = e.Target
			}

			if e.Selection == selection {
				if e.Target == atoms.targets {
					xproto.ChangeProperty(conn, xproto.PropModeReplace, e.Requestor,
						prop, xproto.AtomAtom, 32, uint32(len(targets)), targetData)
				} else if e.Target == atoms.utf8String || e.Target == atoms.string || e.Target == atoms.textPlain {
					xproto.ChangeProperty(
						conn,
						xproto.PropModeReplace,
						e.Requestor,
						prop,
						e.Target,
						8,
						uint32(len(textBytes)),
						textBytes,
					)
				} else if e.Target == atoms.timestamp {
					b := fmt.Appendf(nil, "%d\n", timestamp)
					log.Printf("timestamp: %d, %s", b, string(b))

					xproto.ChangeProperty(
						conn,
						xproto.PropModeReplace,
						e.Requestor,
						prop,
						e.Target,
						8,
						uint32(len(b)),
						b,
					)
				} else {
					// Target not supported
					prop = xproto.AtomNone
				}

				// Send SelectionNotify event
				ev := xproto.SelectionNotifyEvent{
					Time:      e.Time,
					Requestor: e.Requestor,
					Selection: e.Selection,
					Target:    e.Target,
					Property:  prop,
				}
				xproto.SendEvent(conn, false, e.Requestor, xproto.EventMaskNoEvent, string(ev.Bytes()))
			}
		case xproto.SelectionClearEvent:
			if e.Selection == selection {
				fmt.Println("Lost clipboard ownership")
				done = true
			}
		}
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <read|write|readprimary|writeprimary> [text]")
		os.Exit(1)
	}

	log.Printf("Starting... %s", os.Args[1])
	switch os.Args[1] {
	case "read":
		readClipboard(true, false)
	case "readprimary":
		readClipboard(true, true)
	case "write":
		if len(os.Args) < 3 {
			fmt.Println("Usage: go run main.go write <text>")
			os.Exit(1)
		}
		writeClipboard(os.Args[2], false)

		// Keep process running to maintain clipboard ownership
		fmt.Println("Text copied to clipboard. Press Ctrl+C to exit.")
		for {
			time.Sleep(time.Second)
		}
	case "writeprimary":
		if len(os.Args) < 3 {
			fmt.Println("Usage: go run main.go writeprimary <text>")
			os.Exit(1)
		}
		writeClipboard(os.Args[2], true)

		// Keep process running to maintain clipboard ownership
		fmt.Println("Text copied to primary selection. Press Ctrl+C to exit.")
		for {
			time.Sleep(time.Second)
		}
	default:
		fmt.Println("Invalid option. Use 'read', 'write', 'readprimary', or 'writeprimary'")
		os.Exit(1)
	}
}
