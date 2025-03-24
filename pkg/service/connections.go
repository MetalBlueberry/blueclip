package service

import (
	"blueclip/pkg/selections"
	"blueclip/pkg/xclip"
	"bufio"
	"bytes"
	"context"
	"fmt"
	"image/png"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/qeesung/image2ascii/convert"
)

type Server struct {
	*http.Server
	Addr string
}

func (s *Server) ListenAndServe() error {
	ln, err := net.Listen("unix", s.Addr)
	if err != nil {
		return err
	}

	if err := os.Chmod(s.Addr, 0600); err != nil {
		os.Remove(s.Addr)
		return fmt.Errorf("failed to set socket permissions: %v", err)
	}

	return s.Server.Serve(ln)
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.Server.Shutdown(ctx)
}

func NewServer(mux *http.ServeMux) (*Server, error) {
	httpserver := &http.Server{
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	sockDir := filepath.Join(os.TempDir(), "blueclip")
	if err := os.MkdirAll(sockDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create socket directory: %v", err)
	}

	sockPath := filepath.Join(sockDir, "blueclip.sock")

	if err := os.Remove(sockPath); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to remove existing socket: %v", err)
	}

	server := &Server{
		Server: httpserver,
		Addr:   sockPath,
	}

	return server, nil
}

func (s *Service) HandleClear(resp http.ResponseWriter, req *http.Request) {
	query := req.URL.Query()
	typeString := query.Get("type")
	if typeString == "" {
		typeString = "all"
	}

	log.Printf("Clearing selections of type: %s", typeString)
	pattern, err := io.ReadAll(req.Body)
	if err != nil {
		log.Printf("Failed to read pattern: %v", err)
		resp.WriteHeader(http.StatusBadRequest)
		resp.Write([]byte("failed to read pattern"))
		return
	}

	switch typ := selections.SelectionRetentionType(typeString); typ {
	case selections.SelectionRetentionTypeAll,
		selections.SelectionRetentionTypeEphemeral,
		selections.SelectionRetentionTypeImportant:
		s.selections.Clear(string(pattern), typ)
	default:
		resp.WriteHeader(http.StatusBadRequest)
		resp.Write([]byte("invalid type, allowed types are: all, ephemeral, important"))
		return
	}

	resp.WriteHeader(http.StatusOK)
}

func (s *Service) HandleList(resp http.ResponseWriter, req *http.Request) {
	log.Printf("Listing selections")
	s.selections.List(resp)
}

func (s *Service) HandleCopy(resp http.ResponseWriter, req *http.Request) {
	line, err := io.ReadAll(req.Body)
	if err != nil {
		log.Printf("Failed to read line: %v", err)
		return
	}

	selection, ok := s.selections.Copy(line)
	if !ok {
		log.Printf("No match found for line: %s", line)
		return
	}

	clipboardSelections := req.URL.Query()["clipboard-selection"]

	if len(clipboardSelections) == 0 {
		clipboardSelections = []string{string(xclip.ClipboardSelectionClipboard)}
	}

	for _, clipboardSelection := range clipboardSelections {
		log.Printf("Copying selection to clipboard: %s with target: %s", clipboardSelection, selection.Target)
		err = xclip.Cli.Copy(
			req.Context(),
			bytes.NewReader(selection.Content),
			xclip.CopyOptionSelection(xclip.ClipboardSelection(clipboardSelection)),
			xclip.CopyOptionWithTarget(selection.Target),
		)
		if err != nil {
			log.Printf("Failed to copy selection: %v", err)
			resp.WriteHeader(http.StatusBadRequest)
			resp.Write([]byte(fmt.Sprintf("failed to copy selection: %v", err)))
			return
		}
	}

	resp.WriteHeader(http.StatusOK)
}

func (s *Service) HandlePrint(resp http.ResponseWriter, req *http.Request) {
	unindentFlag := req.URL.Query().Get("unindent")
	log.Printf("Handle print with unindent flag: %s", unindentFlag)

	line, err := io.ReadAll(req.Body)
	if err != nil {
		log.Printf("Failed to read line: %v", err)
		return
	}

	selection, ok := s.selections.FindMatch(line)
	if !ok {
		log.Printf("No match found for line: %s", line)
		return
	}

	if unindentFlag == "true" {
		if selection.Target == xclip.ValidTargetImagePng {
			convertOptions := convert.DefaultOptions

			query := req.URL.Query()
			width := query.Get("width")
			height := query.Get("height")
			if width != "" {
				convertOptions.FixedWidth, err = strconv.Atoi(width)
			}
			if height != "" {
				convertOptions.FixedHeight, err = strconv.Atoi(height)
			}

			log.Printf("Preview with %d columns and %d lines", convertOptions.FixedWidth, convertOptions.FixedHeight)

			converter := convert.NewImageConverter()
			imagefile, err := png.Decode(bytes.NewReader(selection.Content))
			if err != nil {
				resp.Write([]byte(fmt.Sprintf("failed to decode image: %v", err)))
				return
			}
			ascii := converter.Image2ASCIIString(imagefile, &convertOptions)

			_, err = resp.Write([]byte(ascii))
			if err != nil {
				log.Printf("Failed to write ascii: %v", err)
			}
		} else {
			unindent(bytes.NewReader(selection.Content), resp)
		}
	} else {
		_, err = resp.Write(selection.Content)
		if err != nil {
			log.Printf("Failed to write selection: %v", err)
		}
	}
	if err != nil {
		log.Printf("Failed to write selection: %v", err)
	}
}

// Very simple unindenter that uses the first line to determine the number of leading spaces
// and removes them from all lines.
func unindent(in io.Reader, out io.Writer) {
	scanner := bufio.NewScanner(in)
	scanner.Split(bufio.ScanLines)
	detectedIndentation := -1

	isIndentation := func(r rune) bool {
		return r != ' ' && r != '\t'
	}

	for scanner.Scan() {
		line := scanner.Bytes()
		if detectedIndentation == -1 {
			detectedIndentation = bytes.IndexFunc(line, isIndentation)
			if detectedIndentation == -1 {
				detectedIndentation = 0
			}
		}
		lineIndentation := bytes.IndexFunc(line, isIndentation)
		if lineIndentation == -1 {
			lineIndentation = 0
		}
		removeMinIndentation := min(detectedIndentation, lineIndentation)
		_, err := out.Write(line[removeMinIndentation:])
		if err != nil {
			log.Printf("Failed to write line: %v", err)
		}
		_, err = out.Write([]byte("\n"))
		if err != nil {
			log.Printf("Failed to write line: %v", err)
		}
	}
}
