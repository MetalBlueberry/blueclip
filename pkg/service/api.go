package service

import (
	"blueclip/pkg/selections"
	"blueclip/pkg/xclip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// The JSON API is meant for programs, mainly AI agents through `blueclip mcp`.
// Entries are addressed by ID and returned with their metadata, unlike the
// fzf oriented endpoints that work with whole lines.

const (
	DefaultListLimit    = 20
	MaxListLimit        = 500
	DefaultListMaxChars = 300
)

type APIEntry struct {
	ID uint64 `json:"id"`
	// Time is RFC 3339, empty when unknown (entries saved before times were recorded).
	Time string `json:"time,omitempty"`
	// Source is "clipboard" for explicit copies, "primary" for highlighted text.
	Source  string `json:"source,omitempty"`
	Kind    string `json:"kind"`
	Current bool   `json:"current,omitempty"`
	Target  string `json:"target"`
	Bytes   int    `json:"bytes"`
	Lines   int    `json:"lines"`
	Content string `json:"content"`
	// Truncated reports that Content was shortened, get the entry for all of it.
	Truncated bool `json:"truncated,omitempty"`
}

type ListEntriesResponse struct {
	// Total is the number of matching entries before limit and offset.
	Total   int        `json:"total"`
	Entries []APIEntry `json:"entries"`
}

type DeleteEntriesRequest struct {
	IDs []uint64 `json:"ids"`
}

type DeleteEntriesResponse struct {
	Deleted  []uint64 `json:"deleted"`
	NotFound []uint64 `json:"not_found"`
}

type CopyEntryRequest struct {
	Selections []string `json:"selections,omitempty"`
}

type SetClipboardRequest struct {
	Text       string   `json:"text"`
	Selections []string `json:"selections,omitempty"`
}

type APIError struct {
	Error string `json:"error"`
}

// ListQuery holds the filters of GET /api/entries. Empty fields don't filter.
type ListQuery struct {
	// Query is split on whitespace, every word must appear, ignoring case.
	Query string
	Regex string
	// Since and Until accept RFC 3339, a local date (2006-01-02), a local
	// date and time (2006-01-02T15:04) or a duration ago (90m, 2h, 3d).
	Since  string
	Until  string
	Source string
	Kind   string
	Limit  int
	Offset int
	// MaxChars truncates each content, 0 uses the default, negative disables it.
	MaxChars int
}

func (q ListQuery) Values() url.Values {
	v := url.Values{}
	set := func(key, value string) {
		if value != "" {
			v.Set(key, value)
		}
	}
	set("query", q.Query)
	set("regex", q.Regex)
	set("since", q.Since)
	set("until", q.Until)
	set("source", q.Source)
	set("kind", q.Kind)
	if q.Limit != 0 {
		v.Set("limit", strconv.Itoa(q.Limit))
	}
	if q.Offset != 0 {
		v.Set("offset", strconv.Itoa(q.Offset))
	}
	if q.MaxChars != 0 {
		v.Set("max_chars", strconv.Itoa(q.MaxChars))
	}
	return v
}

func parseListQuery(v url.Values) (ListQuery, error) {
	q := ListQuery{
		Query:  v.Get("query"),
		Regex:  v.Get("regex"),
		Since:  v.Get("since"),
		Until:  v.Get("until"),
		Source: v.Get("source"),
		Kind:   v.Get("kind"),
	}
	for key, dst := range map[string]*int{"limit": &q.Limit, "offset": &q.Offset, "max_chars": &q.MaxChars} {
		if raw := v.Get(key); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil {
				return q, fmt.Errorf("%s must be an integer: %v", key, err)
			}
			*dst = n
		}
	}
	return q, nil
}

func (q ListQuery) filter(now time.Time) (selections.Filter, error) {
	f := selections.Filter{Terms: strings.Fields(q.Query)}
	var err error
	if q.Regex != "" {
		if f.Regex, err = regexp.Compile(q.Regex); err != nil {
			return f, fmt.Errorf("invalid regex: %v", err)
		}
	}
	if f.Since, err = parseTime(q.Since, now); err != nil {
		return f, fmt.Errorf("invalid since: %v", err)
	}
	if f.Until, err = parseTime(q.Until, now); err != nil {
		return f, fmt.Errorf("invalid until: %v", err)
	}
	switch xclip.ClipboardSelection(q.Source) {
	case "", xclip.ClipboardSelectionClipboard, xclip.ClipboardSelectionPrimary, xclip.ClipboardSelectionSecondary:
		f.Source = xclip.ClipboardSelection(q.Source)
	default:
		return f, fmt.Errorf("invalid source %q, valid values are clipboard, primary and secondary", q.Source)
	}
	switch selections.Kind(q.Kind) {
	case "", selections.KindEphemeral, selections.KindImportant:
		f.Kind = selections.Kind(q.Kind)
	default:
		return f, fmt.Errorf("invalid kind %q, valid values are ephemeral and important", q.Kind)
	}
	return f, nil
}

func parseTime(raw string, now time.Time) (time.Time, error) {
	if raw == "" {
		return time.Time{}, nil
	}
	if strings.HasSuffix(raw, "d") {
		if days, err := strconv.Atoi(strings.TrimSuffix(raw, "d")); err == nil {
			return now.Add(-time.Duration(days) * 24 * time.Hour), nil
		}
	}
	if d, err := time.ParseDuration(raw); err == nil {
		return now.Add(-d), nil
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t, nil
	}
	for _, layout := range []string{"2006-01-02T15:04", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, raw, now.Location()); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("%q is not RFC 3339, 2006-01-02, 2006-01-02T15:04 or a duration like 2h or 3d", raw)
}

func toAPIEntry(e selections.Entry, maxChars int) APIEntry {
	out := APIEntry{
		ID:      e.ID,
		Source:  string(e.Source),
		Kind:    string(e.Kind),
		Current: e.Current,
		Target:  string(e.Target),
		Bytes:   len(e.Content),
		Lines:   countLines(e.Content),
		Content: string(e.Content),
	}
	if !e.Time.IsZero() {
		out.Time = e.Time.Format(time.RFC3339)
	}
	if maxChars >= 0 && utf8.RuneCountInString(out.Content) > maxChars {
		out.Content = string([]rune(out.Content)[:maxChars])
		out.Truncated = true
	}
	return out
}

func countLines(content []byte) int {
	lines := bytes.Count(content, []byte("\n"))
	if len(content) > 0 && content[len(content)-1] != '\n' {
		lines++
	}
	return lines
}

func writeJSON(resp http.ResponseWriter, status int, v any) {
	resp.Header().Set("Content-Type", "application/json")
	resp.WriteHeader(status)
	if err := json.NewEncoder(resp).Encode(v); err != nil {
		log.Printf("Failed to write response: %v", err)
	}
}

func writeError(resp http.ResponseWriter, status int, format string, args ...any) {
	writeJSON(resp, status, APIError{Error: fmt.Sprintf(format, args...)})
}

func pathID(req *http.Request) (uint64, error) {
	id, err := strconv.ParseUint(req.PathValue("id"), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid id %q", req.PathValue("id"))
	}
	return id, nil
}

func validSelections(names []string) error {
	for _, name := range names {
		switch xclip.ClipboardSelection(name) {
		case xclip.ClipboardSelectionClipboard, xclip.ClipboardSelectionPrimary, xclip.ClipboardSelectionSecondary:
		default:
			return fmt.Errorf("invalid selection %q, valid values are clipboard, primary and secondary", name)
		}
	}
	return nil
}

func (s *Service) HandleListEntries(resp http.ResponseWriter, req *http.Request) {
	q, err := parseListQuery(req.URL.Query())
	if err != nil {
		writeError(resp, http.StatusBadRequest, "%v", err)
		return
	}
	f, err := q.filter(time.Now())
	if err != nil {
		writeError(resp, http.StatusBadRequest, "%v", err)
		return
	}

	limit := q.Limit
	switch {
	case limit <= 0:
		limit = DefaultListLimit
	case limit > MaxListLimit:
		limit = MaxListLimit
	}
	maxChars := q.MaxChars
	if maxChars == 0 {
		maxChars = DefaultListMaxChars
	}

	entries := s.selections.Entries(f)
	out := ListEntriesResponse{Total: len(entries), Entries: []APIEntry{}}
	if q.Offset < 0 {
		q.Offset = 0
	}
	for i := q.Offset; i < len(entries) && len(out.Entries) < limit; i++ {
		out.Entries = append(out.Entries, toAPIEntry(entries[i], maxChars))
	}
	writeJSON(resp, http.StatusOK, out)
}

func (s *Service) HandleGetEntry(resp http.ResponseWriter, req *http.Request) {
	id, err := pathID(req)
	if err != nil {
		writeError(resp, http.StatusBadRequest, "%v", err)
		return
	}
	e, ok := s.selections.Entry(id)
	if !ok {
		writeError(resp, http.StatusNotFound, "no entry with id %d", id)
		return
	}
	writeJSON(resp, http.StatusOK, toAPIEntry(e, -1))
}

func (s *Service) HandleDeleteEntries(resp http.ResponseWriter, req *http.Request) {
	var body DeleteEntriesRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeError(resp, http.StatusBadRequest, "invalid body: %v", err)
		return
	}
	if len(body.IDs) == 0 {
		writeError(resp, http.StatusBadRequest, "ids is empty")
		return
	}

	deleted, notFound := s.selections.Delete(body.IDs)
	log.Printf("Deleted %d entries, %d not found", len(deleted), len(notFound))
	if len(deleted) > 0 {
		if err := s.persist(); err != nil {
			writeError(resp, http.StatusInternalServerError, "failed to save selections: %v", err)
			return
		}
	}
	writeJSON(resp, http.StatusOK, DeleteEntriesResponse{Deleted: deleted, NotFound: notFound})
}

func (s *Service) HandleCopyEntry(resp http.ResponseWriter, req *http.Request) {
	id, err := pathID(req)
	if err != nil {
		writeError(resp, http.StatusBadRequest, "%v", err)
		return
	}
	var body CopyEntryRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		writeError(resp, http.StatusBadRequest, "invalid body: %v", err)
		return
	}
	if err := validSelections(body.Selections); err != nil {
		writeError(resp, http.StatusBadRequest, "%v", err)
		return
	}

	sel, ok := s.selections.CopyByID(id)
	if !ok {
		writeError(resp, http.StatusNotFound, "no entry with id %d", id)
		return
	}
	if err := copyToClipboard(req.Context(), sel.Content, sel.Target, body.Selections); err != nil {
		writeError(resp, http.StatusInternalServerError, "failed to copy: %v", err)
		return
	}
	if err := s.persist(); err != nil {
		writeError(resp, http.StatusInternalServerError, "failed to save selections: %v", err)
		return
	}
	e, _ := s.selections.Entry(id)
	writeJSON(resp, http.StatusOK, toAPIEntry(e, DefaultListMaxChars))
}

// HandleSetClipboard copies new text and records it in the history right
// away, so the caller gets its ID without waiting for the watcher.
func (s *Service) HandleSetClipboard(resp http.ResponseWriter, req *http.Request) {
	var body SetClipboardRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeError(resp, http.StatusBadRequest, "invalid body: %v", err)
		return
	}
	if body.Text == "" {
		writeError(resp, http.StatusBadRequest, "text is empty")
		return
	}
	if err := validSelections(body.Selections); err != nil {
		writeError(resp, http.StatusBadRequest, "%v", err)
		return
	}

	content := []byte(body.Text)
	if err := copyToClipboard(req.Context(), content, xclip.ValidTargetUTF8_STRING, body.Selections); err != nil {
		writeError(resp, http.StatusInternalServerError, "failed to copy: %v", err)
		return
	}

	source := xclip.ClipboardSelectionClipboard
	if len(body.Selections) > 0 {
		source = xclip.ClipboardSelection(body.Selections[0])
	}
	stored := s.selections.Add(selections.Selection{Selection: xclip.Selection{
		Content: content,
		Target:  xclip.ValidTargetUTF8_STRING,
		Source:  source,
	}})
	if err := s.persist(); err != nil {
		writeError(resp, http.StatusInternalServerError, "failed to save selections: %v", err)
		return
	}
	e, _ := s.selections.Entry(stored.ID)
	writeJSON(resp, http.StatusOK, toAPIEntry(e, DefaultListMaxChars))
}
