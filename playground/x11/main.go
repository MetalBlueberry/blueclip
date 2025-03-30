package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
)

const (
	timestampAtomName = "TIMESTAMP"
	clipboardAtomName = "CLIPBOARD"
	primaryAtomName   = "PRIMARY"

	targetsAtomName    = "TARGETS"
	utf8StringAtomName = "UTF8_STRING"
	textPlainAtomName  = "text/plain"
	stringAtomName     = "STRING"
	imagePngAtomName   = "image/png"
	imageJpegAtomName  = "image/jpeg"
	imageBmpAtomName   = "image/bmp"
	incrAtomName       = "INCR"
)

type Atom struct {
	Name string
	Atom xproto.Atom
}

// AtomInfo holds information about well-known atoms
type AtomInfo struct {
	timestamp  Atom
	clipboard  Atom
	primary    Atom
	targets    Atom
	utf8String Atom
	textPlain  Atom
	string     Atom
	imagePng   Atom
	imageJpeg  Atom
	imageBmp   Atom
	incr       Atom
}

func getAtom(conn *xgb.Conn, name string) Atom {
	reply, err := xproto.InternAtom(conn, false, uint16(len(name)), name).Reply()
	if err != nil {
		log.Fatalf("Failed to intern atom %s: %v", name, err)
	}
	return Atom{
		Name: name,
		Atom: reply.Atom,
	}
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
		[]uint32{xproto.EventMaskStructureNotify},
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
func getOwner(conn *xgb.Conn, clipboard Atom) xproto.Window {
	reply, err := xproto.GetSelectionOwner(conn, clipboard.Atom).Reply()
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
func getSupportedMimes(conn *xgb.Conn, win xproto.Window, selection Atom, targets Atom) []Atom {
	log.Print(win, selection, targets, xproto.TimeCurrentTime)
	xproto.ConvertSelection(conn, win, selection.Atom, targets.Atom, targets.Atom, xproto.TimeCurrentTime)

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
				log.Printf("Property is none")
				return nil
			}

			reply, err := xproto.GetProperty(conn, false, win, selNotify.Property, xproto.AtomAtom, 0, (1<<32)-1).Reply()
			if err != nil {
				log.Printf("Failed to get targets: %v", err)
				return nil
			}

			log.Printf("targets: %v", reply.Value)
			// Convert the []byte to []xproto.Atom
			formats := make([]Atom, len(reply.Value)/4)
			for i := 0; i < len(formats); i++ {
				name, err := xproto.GetAtomName(conn, xproto.Atom(xgb.Get32(reply.Value[i*4:]))).Reply()
				if err != nil {
					log.Printf("Failed to get atom name: %v", err)
					return nil
				}
				formats[i] = Atom{
					Name: name.Name,
					Atom: xproto.Atom(xgb.Get32(reply.Value[i*4:])),
				}
			}

			// Delete the property to let the owner know we've read it
			err = xproto.DeletePropertyChecked(conn, win, selNotify.Property).Check()
			if err != nil {
				log.Printf("Failed to delete property: %v", err)
			}

			return formats
		} else {
			log.Printf("Unknown event: %v", ev)
		}
	}
}

// getPriorityFormat chooses the best available format from the supported formats
func getPriorityFormat(supportedFormats []Atom, atoms AtomInfo) []Atom {
	// Priority order for formats
	priorities := []xproto.Atom{
		atoms.utf8String.Atom,
		atoms.textPlain.Atom,
		atoms.string.Atom,
		atoms.imagePng.Atom,
		atoms.imageJpeg.Atom,
		atoms.imageBmp.Atom,
		atoms.timestamp.Atom,
	}
	results := []Atom{}

	for _, prio := range priorities {
		for _, format := range supportedFormats {
			if format.Atom == prio {
				results = append(results, format)
			}
		}
	}

	return results
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
	availableFormats := getSupportedMimes(conn, win, selection, atoms.targets)

	if len(availableFormats) == 0 {
		fmt.Println("No formats available in clipboard")
		return
	}

	fmt.Println("Available formats:")
	for _, f := range availableFormats {
		fmt.Printf("  - %s\n", f.Name)
	}

	// Choose the format to request based on priority
	supportedFormats := getPriorityFormat(availableFormats, atoms)
	if len(supportedFormats) == 0 {
		fmt.Println("No supported format found in clipboard")
		return
	}
	log.Printf("Supported formats:")
	for _, f := range supportedFormats {
		fmt.Printf("  - %s\n", f.Name)
	}

	for _, targetFormat := range supportedFormats {
		fmt.Printf("Using format: %s\n", targetFormat.Name)

		// Request the content with the chosen format
		xproto.ConvertSelection(conn, win, selection.Atom, targetFormat.Atom, targetFormat.Atom, xproto.TimeCurrentTime)

		// Wait for the data
		for {
			ev, err := conn.WaitForEvent()
			if err != nil {
				log.Printf("Event error: %v", err)
				continue
			}
			log.Printf("Received event: %v", ev)

			if selNotify, ok := ev.(xproto.SelectionNotifyEvent); ok {
				if selNotify.Property == xproto.AtomNone {
					fmt.Println("Selection conversion failed")
				}

				name, err := xproto.GetAtomName(conn, selNotify.Target).Reply()
				if err != nil {
					log.Printf("Failed to get atom name: %v", err)
				}
				fmt.Printf("Target: %s\n", name.Name)

				name, err = xproto.GetAtomName(conn, selNotify.Property).Reply()
				if err != nil {
					log.Printf("Failed to get atom name: %v", err)
				}
				fmt.Printf("Property: %s\n", name.Name)

				// Check if it's an incremental transfer
				if isIncrementalTransfer(conn, win, selNotify.Property, atoms.incr.Atom) {
					xproto.DeleteProperty(conn, win, selNotify.Property)
					fmt.Println("Incremental transfer not supported")
					return
				}

				// Handle based on format
				switch targetFormat {
				case atoms.utf8String, atoms.string, atoms.textPlain:
					reply, err := xproto.GetProperty(conn, false, win, selNotify.Property, xproto.AtomAny, 0, (1<<32)-1).Reply()
					if err != nil {
						log.Fatalf("Failed to get property: %v", err)
					} else {
						fmt.Println("Text content:", string(reply.Value))
					}
				case atoms.imagePng:
					reply, err := xproto.GetProperty(conn, false, win, selNotify.Property, selNotify.Target, 0, (1<<32)-1).Reply()
					if err != nil {
						log.Fatalf("Failed to get property: %v", err)
					} else {
						fmt.Printf("PNG image data: %d bytes\n", len(reply.Value))
					}
				case atoms.timestamp:
					reply, err := xproto.GetProperty(conn, false, win, selNotify.Property, xproto.AtomAny, 0, (1<<32)-1).Reply()
					if err != nil {
						log.Fatalf("Failed to get property: %v", err)
					}
					if len(reply.Value) >= 4 {
						timestamp := xgb.Get32(reply.Value)
						fmt.Printf("Timestamp: %d\n", timestamp)
					} else {
						fmt.Println("Invalid timestamp data length")
					}
				default:
					fmt.Printf("Unknown format data: %d bytes\n", targetFormat)
				}

				// Delete the property to let the owner know we've read it
				xproto.DeleteProperty(conn, win, selNotify.Property)
				break
			}
		}
	}
}

// getTimestamp gets a valid timestamp by appending to a temporary property
func getTimestamp(conn *xgb.Conn, win xproto.Window) xproto.Timestamp {
	// Create a temporary property
	tempAtom := getAtom(conn, "TEMP")

	// Append empty data to get a timestamp
	xproto.ChangeProperty(conn, xproto.PropModeAppend, win, tempAtom.Atom, xproto.AtomString, 8, 0, []byte{})

	// Wait for PropertyNotify event
	for {
		ev, err := conn.WaitForEvent()
		if err != nil {
			log.Printf("Event error: %v", err)
			continue
		}

		if propNotify, ok := ev.(xproto.PropertyNotifyEvent); ok {
			if propNotify.Window == win && propNotify.Atom == tempAtom.Atom {
				// Clean up the temporary property
				xproto.DeleteProperty(conn, win, tempAtom.Atom)
				return propNotify.Time
			}
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

	// Get a valid timestamp
	timestamp := getTimestamp(conn, win)
	log.Printf("timestamp: %d", timestamp)

	// Take ownership of clipboard with the proper timestamp
	xproto.SetSelectionOwner(conn, win, selection.Atom, timestamp)
	owner, err := xproto.GetSelectionOwner(conn, selection.Atom).Reply()
	if err != nil {
		log.Fatalf("Failed to get selection owner: %v", err)
	}
	if owner.Owner != win {
		log.Fatalf("Failed to take clipboard ownership, owner: %d, expected: %d", owner.Owner, win)
	}
	log.Printf("Clipboard ownership taken")

	// Define the supported targets
	targets := []Atom{atoms.timestamp, atoms.targets, atoms.utf8String, atoms.string, atoms.textPlain}
	targetData := make([]byte, len(targets)*4)
	for i, a := range targets {
		xgb.Put32(targetData[i*4:], uint32(a.Atom))
	}

	// Event handling loop
	textBytes := []byte(text)
	done := false
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

			if e.Selection == selection.Atom {
				if e.Target == atoms.targets.Atom {
					xproto.ChangeProperty(conn, xproto.PropModeReplace, e.Requestor,
						prop, xproto.AtomAtom, 32, uint32(len(targets)), targetData)
				} else if e.Target == atoms.utf8String.Atom || e.Target == atoms.string.Atom || e.Target == atoms.textPlain.Atom {
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
				} else if e.Target == atoms.timestamp.Atom {
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
			if e.Selection == selection.Atom {
				fmt.Println("Lost clipboard ownership")
				done = true
			}
		default:
			log.Printf("Unknown event: %v", e)
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

	case "writeprimary":
		if len(os.Args) < 3 {
			fmt.Println("Usage: go run main.go writeprimary <text>")
			os.Exit(1)
		}
		writeClipboard(os.Args[2], true)
	case "watch":
		watcher, err := NewWatcher()
		if err != nil {
			log.Fatalf("Failed to create watcher: %v", err)
		}
		watcher.Watch(context.Background())
	default:
		fmt.Println("Invalid option. Use 'read', 'write', 'readprimary', or 'writeprimary'")
		os.Exit(1)
	}
}

type Selection struct {
	Properties []Property
}

type Property struct {
	Type    Atom
	Content []byte
}

type Watcher struct {
	conn  *xgb.Conn
	atoms AtomInfo
	win   xproto.Window
}

func NewWatcher() (*Watcher, error) {
	conn, err := xgb.NewConn()
	if err != nil {
		return nil, err
	}

	atoms := initAtoms(conn)
	win := createWindow(conn)

	return &Watcher{
		conn:  conn,
		atoms: atoms,
		win:   win,
	}, nil
}

func (w *Watcher) Close() error {
	defer w.conn.Close()
	return xproto.DestroyWindowChecked(w.conn, w.win).Check()
}

func (w *Watcher) Read(clipboard Atom) (*Selection, error) {
	// Get owner window name
	owner := getOwner(w.conn, clipboard)
	ownerName := getOwnerName(w.conn, owner)
	fmt.Println("Clipboard owner:", ownerName)

	// First get the supported target formats
	availableFormats := getSupportedMimes(w.conn, w.win, clipboard, w.atoms.targets)
	if len(availableFormats) == 0 {
		return nil, errors.New("no formats available in clipboard")
	}

	fmt.Println("Available formats:")
	for _, f := range availableFormats {
		fmt.Printf("  - %s\n", f.Name)
	}

	// Choose the format to request based on priority
	supportedFormats := getPriorityFormat(availableFormats, w.atoms)
	if len(supportedFormats) == 0 {
		return nil, errors.New("no supported format found in clipboard")
	}
	log.Printf("Supported formats:")
	for _, f := range supportedFormats {
		fmt.Printf("  - %s\n", f.Name)
	}

	selection := &Selection{
		Properties: []Property{},
	}

	for _, targetFormat := range supportedFormats {
		fmt.Printf("Using format: %s\n", targetFormat.Name)

		// Request the content with the chosen format
		xproto.ConvertSelection(w.conn, w.win, clipboard.Atom, targetFormat.Atom, targetFormat.Atom, xproto.TimeCurrentTime)

		// Wait for the data
		for {
			ev, err := w.conn.WaitForEvent()
			if err != nil {
				log.Printf("Event error: %v", err)
				continue
			}
			log.Printf("Received event: %v", ev)

			if selNotify, ok := ev.(xproto.SelectionNotifyEvent); ok {
				if selNotify.Property == xproto.AtomNone {
					fmt.Println("Selection conversion failed")
				}

				// name, err := xproto.GetAtomName(w.conn, selNotify.Target).Reply()
				// if err != nil {
				// 	log.Printf("Failed to get atom name: %v", err)
				// }
				// fmt.Printf("Target: %s\n", name.Name)

				// name, err = xproto.GetAtomName(w.conn, selNotify.Property).Reply()
				// if err != nil {
				// 	log.Printf("Failed to get atom name: %v", err)
				// }
				// fmt.Printf("Property: %s\n", name.Name)

				// Check if it's an incremental transfer
				if isIncrementalTransfer(w.conn, w.win, selNotify.Property, w.atoms.incr.Atom) {
					xproto.DeleteProperty(w.conn, w.win, selNotify.Property)
					fmt.Println("Incremental transfer not supported")
					return nil, errors.New("incremental transfer not supported")
				}

				// Handle based on format
				switch targetFormat {
				case w.atoms.utf8String, w.atoms.string, w.atoms.textPlain:
					reply, err := xproto.GetProperty(w.conn, false, w.win, selNotify.Property, xproto.AtomAny, 0, (1<<32)-1).Reply()
					if err != nil {
						log.Fatalf("Failed to get property: %v", err)
					} else {
						selection.Properties = append(selection.Properties, Property{
							Type:    targetFormat,
							Content: reply.Value,
						})
					}
				case w.atoms.imagePng:
					reply, err := xproto.GetProperty(w.conn, false, w.win, selNotify.Property, selNotify.Target, 0, (1<<32)-1).Reply()
					if err != nil {
						log.Fatalf("Failed to get property: %v", err)
					} else {
						selection.Properties = append(selection.Properties, Property{
							Type:    targetFormat,
							Content: reply.Value,
						})
					}
				case w.atoms.timestamp:
					reply, err := xproto.GetProperty(w.conn, false, w.win, selNotify.Property, xproto.AtomAny, 0, (1<<32)-1).Reply()
					if err != nil {
						log.Fatalf("Failed to get property: %v", err)
					}
					if len(reply.Value) >= 4 {
						selection.Properties = append(selection.Properties, Property{
							Type:    targetFormat,
							Content: reply.Value,
						})
					} else {
						fmt.Println("Invalid timestamp data length")
					}
				default:
					fmt.Printf("Unknown format data: %d bytes\n", targetFormat)
				}

				// Delete the property to let the owner know we've read it
				xproto.DeleteProperty(w.conn, w.win, selNotify.Property)
				break
			}
			return nil, errors.New("unknown event")
		}
	}
	return selection, nil
}

func (w *Watcher) Write(clipboard Atom, selection *Selection) error {

	// Choose clip based on parameter
	clip := w.atoms.clipboard
	if clipboard == w.atoms.primary {
		clip = w.atoms.primary
	}
	// Take ownership of clipboard with the proper timestamp
	xproto.SetSelectionOwner(w.conn, w.win, w.atoms.clipboard.Atom, xproto.TimeCurrentTime)
	owner, err := xproto.GetSelectionOwner(w.conn, w.atoms.clipboard.Atom).Reply()
	if err != nil {
		log.Fatalf("Failed to get selection owner: %v", err)
	}
	if owner.Owner != w.win {
		log.Fatalf("Failed to take clipboard ownership, owner: %d, expected: %d", owner.Owner, w.win)
	}
	log.Printf("Clipboard ownership taken")

	// Define the supported targets
	targets := []Atom{w.atoms.timestamp, w.atoms.targets, w.atoms.utf8String, w.atoms.string, w.atoms.textPlain}
	targetData := make([]byte, len(targets)*4)
	for i, a := range targets {
		xgb.Put32(targetData[i*4:], uint32(a.Atom))
	}

	done := false
	for !done {
		ev, err := w.conn.WaitForEvent()
		if err != nil {
			fmt.Printf("Event error: %v\n", err)
			continue
		}

		switch e := ev.(type) {
		case xproto.SelectionRequestEvent:
			log.Printf("SelectionRequestEvent: %v", e)
			if e.Selection != clip.Atom {
				log.Printf("Selection not for this clipboard: %v", e.Selection)
				continue
			}
			var prop xproto.Atom = e.Property
			if prop == xproto.AtomNone {
				prop = e.Target
			}

			var found *Property
			for _, p := range selection.Properties {
				if e.Target == p.Type.Atom {
					found = &p
					break
				}
			}
			if e.Target == w.atoms.targets.Atom {
				found = &Property{Type: w.atoms.targets}
			}
			if found == nil {
				log.Printf("Unknown format: %v", e.Target)
				xproto.SendEvent(w.conn, false, e.Requestor, xproto.EventMaskNoEvent, string(xproto.SelectionNotifyEvent{
					Time:      e.Time,
					Requestor: e.Requestor,
					Selection: e.Selection,
					Target:    e.Target,
					Property:  xproto.AtomNone,
				}.Bytes()))
				continue
			}
			log.Printf("Found property: %s", found.Type)
			switch found.Type.Name {
			case "TIMESTAMP":
				b := fmt.Appendf(nil, "%d\n", found.Timestamp())
				log.Printf("timestamp: %d, %s", b, string(b))

				xproto.ChangeProperty(
					w.conn,
					xproto.PropModeReplace,
					e.Requestor,
					prop,
					e.Target,
					8,
					uint32(len(found.Content)),
					found.Content,
				)
			case "TARGETS":
				log.Print("Serving targets:")
				xproto.ChangeProperty(w.conn, xproto.PropModeReplace, e.Requestor,
					prop, xproto.AtomAtom, 32, uint32(len(targets)), targetData)
			case "UTF8_STRING", "STRING", "text/plain":
				xproto.ChangeProperty(
					w.conn,
					xproto.PropModeReplace,
					e.Requestor,
					prop,
					e.Target,
					8,
					uint32(len(found.Content)),
					found.Content,
				)
			}

			// Send SelectionNotify event
			ev := xproto.SelectionNotifyEvent{
				Time:      e.Time,
				Requestor: e.Requestor,
				Selection: e.Selection,
				Target:    e.Target,
				Property:  prop,
			}
			xproto.SendEvent(w.conn, false, e.Requestor, xproto.EventMaskNoEvent, string(ev.Bytes()))
		case xproto.SelectionClearEvent:
			if e.Selection == clip.Atom {
				fmt.Println("Lost clipboard ownership")
				done = true
			}
		default:
			log.Printf("Unknown event: %v", e)
		}
	}
	return nil
}

func (w *Watcher) Watch(ctx context.Context) error {
	for {
		log.Printf("Reading clipboard")
		sel, err := w.Read(w.atoms.clipboard)
		if err != nil {
			log.Printf("Failed to read clipboard: %v", err)
			time.Sleep(1 * time.Second)
			continue
		}

		log.Printf("Serving clipboard with content")
		for _, p := range sel.Properties {
			log.Printf("  - %s: %s", p.Type.Name, p.String())
		}
		err = w.Write(w.atoms.clipboard, sel)
		if err != nil {
			log.Printf("Failed to write clipboard: %v", err)
			continue
		}
		log.Printf("Lost selection, reading and recovering")
	}

	return nil
}

func (p *Property) Timestamp() uint32 {
	if p.Type.Name != "TIMESTAMP" {
		panic("Property is not a timestamp")
	}
	return xgb.Get32(p.Content)
}

func (p *Property) String() string {
	switch p.Type.Name {
	case "UTF8_STRING":
		return string(p.Content)
	case "STRING":
		return string(p.Content)
	case "text/plain":
		return string(p.Content)
	case "TIMESTAMP":
		return fmt.Sprintf("%d", p.Timestamp())
	case "image/png":
		return fmt.Sprintf("PNG image: %d bytes", len(p.Content))
	case "image/jpeg":
		return fmt.Sprintf("JPEG image: %d bytes", len(p.Content))
	case "image/bmp":
		return fmt.Sprintf("BMP image: %d bytes", len(p.Content))
	default:
		panic(fmt.Sprintf("Unknown property type: %s", p.Type.Name))
	}
}
