package service

import (
	"blueclip/pkg/db"
	"blueclip/pkg/selections"
	"blueclip/pkg/xclip"
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

type Service struct {
	db *db.FileDB

	lock       sync.Mutex
	selections *selections.Set
}

func NewService(db *db.FileDB) *Service {
	return &Service{
		db:         db,
		selections: selections.NewSelections(),
	}
}

func (s *Service) runListener(ctx context.Context) error {
	log.Println("Starting service...")
	mux := http.NewServeMux()
	mux.HandleFunc("/copy", s.HandleCopy)
	mux.HandleFunc("/print", s.HandlePrint)
	mux.HandleFunc("/list", s.HandleList)
	mux.HandleFunc("/clear", s.HandleClear)

	server, err := NewServer(mux)
	if err != nil {
		return fmt.Errorf("failed to create server: %v", err)
	}

	go func() {
		log.Printf("Listening on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Server error: %v", err)
			// how to handle this?
			panic(err)
		}
	}()

	return nil
}

func (s *Service) Run(ctx context.Context) error {

	log.Println("Loading previous selections...")
	selections, err := s.db.Load()
	if err != nil {
		log.Fatalf("Failed to load selections at %s: %v", s.db.Path, err)
	}
	s.selections = selections

	ch := xclip.Cli.Watch(
		ctx,
		xclip.WatchOptionWithTargets([]xclip.ValidTarget{xclip.ValidTargetUTF8_STRING}),
		xclip.WatchOptionWithClipboardSelection(xclip.ClipboardSelectionClipboard),
		xclip.WatchOptionWithFrequency(200*time.Millisecond),
	)

	err = s.runListener(ctx)
	if err != nil {
		return fmt.Errorf("failed to run listener: %v", err)
	}

	// Handle clipboard changes
	log.Printf("Watching clipboard for changes...")

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case data := <-ch:
			s.handleClipboardChange(ctx, data)
		}
	}
}
func (s *Service) handleClipboardChange(ctx context.Context, data xclip.Selection) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.selections.Add(selections.Selection{
		Selection: data,
	})

	err := s.db.Save(s.selections)
	if err != nil {
		log.Printf("Failed to save selections: %s", err)
	}
}
