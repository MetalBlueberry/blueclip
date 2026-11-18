package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type Client struct {
	*http.Client
}

func NewClient(socket string) *Client {
	SocketTransport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			d := net.Dialer{}
			return d.DialContext(ctx, "unix", socket)
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	client := &Client{
		Client: &http.Client{
			Transport: SocketTransport,
		},
	}
	return client
}

func (c *Client) List(ctx context.Context) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://blueclip/list", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	resp, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}

	return resp, nil
}

type PrintOption func(*http.Request)

func PrintWithUnindent(unindent bool) PrintOption {
	return func(req *http.Request) {
		q := req.URL.Query()
		q.Set("unindent", strconv.FormatBool(unindent))
		req.URL.RawQuery = q.Encode()
	}
}

func (c *Client) Print(ctx context.Context, in io.Reader, opts ...PrintOption) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://blueclip/print", in)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	for _, opt := range opts {
		opt(req)
	}

	resp, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	return resp, nil
}

type CopyOption func(*http.Request)

func CopyWithClipboardSelection(selection []string) CopyOption {
	return func(req *http.Request) {
		q := req.URL.Query()
		for _, s := range selection {
			q.Add("clipboard-selection", string(s))
		}
		req.URL.RawQuery = q.Encode()
	}
}

func (c *Client) Copy(ctx context.Context, in io.Reader, opts ...CopyOption) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://blueclip/copy", in)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	for _, opt := range opts {
		opt(req)
	}

	resp, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to copy request: %v", err)
	}

	return resp, nil
}

type ClearOption func(*http.Request)

func ClearWithType(t string) ClearOption {
	return func(req *http.Request) {
		q := req.URL.Query()
		q.Set("type", t)
		req.URL.RawQuery = q.Encode()
	}
}

func ClearWithAll(all bool) ClearOption {
	return func(req *http.Request) {
		q := req.URL.Query()
		q.Set("all", strconv.FormatBool(all))
		req.URL.RawQuery = q.Encode()
	}
}

func (c *Client) Clear(ctx context.Context, in io.Reader, opts ...ClearOption) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://blueclip/clear", in)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	for _, opt := range opts {
		opt(req)
	}

	resp, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to clear request: %v", err)
	}

	return resp, nil
}

func (c *Client) doJSON(ctx context.Context, method, path string, query url.Values, body, out any) error {
	var in io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to encode request: %v", err)
		}
		in = bytes.NewReader(buf)
	}

	u := "http://blueclip" + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u, in)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.Do(req)
	if err != nil {
		return fmt.Errorf("failed to reach the blueclip server, is it running? (systemctl --user status blueclip): %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		var apiErr APIError
		if err := json.NewDecoder(resp.Body).Decode(&apiErr); err == nil && apiErr.Error != "" {
			return errors.New(apiErr.Error)
		}
		return fmt.Errorf("unexpected response: %s", resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("failed to decode response: %v", err)
	}
	return nil
}

func (c *Client) ListEntries(ctx context.Context, q ListQuery) (ListEntriesResponse, error) {
	var out ListEntriesResponse
	err := c.doJSON(ctx, http.MethodGet, "/api/entries", q.Values(), nil, &out)
	return out, err
}

func (c *Client) GetEntry(ctx context.Context, id uint64) (APIEntry, error) {
	var out APIEntry
	err := c.doJSON(ctx, http.MethodGet, "/api/entries/"+strconv.FormatUint(id, 10), nil, nil, &out)
	return out, err
}

func (c *Client) DeleteEntries(ctx context.Context, ids []uint64) (DeleteEntriesResponse, error) {
	var out DeleteEntriesResponse
	err := c.doJSON(ctx, http.MethodPost, "/api/entries/delete", nil, DeleteEntriesRequest{IDs: ids}, &out)
	return out, err
}

func (c *Client) CopyEntry(ctx context.Context, id uint64, selections []string) (APIEntry, error) {
	var out APIEntry
	err := c.doJSON(ctx, http.MethodPost, "/api/entries/"+strconv.FormatUint(id, 10)+"/copy", nil, CopyEntryRequest{Selections: selections}, &out)
	return out, err
}

func (c *Client) SetClipboard(ctx context.Context, text string, selections []string) (APIEntry, error) {
	var out APIEntry
	err := c.doJSON(ctx, http.MethodPost, "/api/clipboard", nil, SetClipboardRequest{Text: text, Selections: selections}, &out)
	return out, err
}
