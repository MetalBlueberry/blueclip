package service

import (
	"blueclip/pkg/selections"
	"blueclip/pkg/xclip"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	HeaderAction = "action"
	HeaderStatus = "status"
)

const (
	ActionList  = "list"
	ActionCopy  = "copy"
	ActionPrint = "print"
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
	log.Printf("Clearing selections")
	resp.Header().Set(HeaderStatus, "success")
	s.selections = selections.NewSelections()
}

func (s *Service) HandleList(resp http.ResponseWriter, req *http.Request) {
	log.Printf("Listing selections")
	resp.Header().Set(HeaderStatus, "success")
	s.selections.List(resp)
}

func (s *Service) HandleCopy(resp http.ResponseWriter, req *http.Request) {
	resp.Header().Set(HeaderStatus, "success")
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

	err = xclip.Cli.Copy(
		bytes.NewReader(selection.Content),
		xclip.CopyOptionSelection(xclip.ClipboardSelectionClipboard),
	)
	if err != nil {
		log.Printf("Failed to copy selection: %v", err)
		return
	}
}

func (s *Service) HandlePrint(resp http.ResponseWriter, req *http.Request) {
	log.Printf("print selection")
	resp.Header().Set(HeaderStatus, "success")
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

	_, err = resp.Write(selection.Content)
	if err != nil {
		log.Printf("Failed to write selection: %v", err)
	}
}
