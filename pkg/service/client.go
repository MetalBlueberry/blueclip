package service

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
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
