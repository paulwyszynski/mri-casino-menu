// Package fetch downloads the menu page.
package fetch

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

const maxBody = 5 << 20 // 5 MiB; the page is ~90 KB

// HTTP fetches URL with a plain GET.
type HTTP struct {
	URL    string
	Client *http.Client
}

func New(url string) *HTTP {
	return &HTTP{URL: url, Client: &http.Client{Timeout: 20 * time.Second}}
}

func (h *HTTP) Fetch(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}
	req.Header.Set("User-Agent", "menubot/1.0 (+https://github.com/paulwyszynski/mri-casino-menu)")
	resp, err := h.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: unexpected status %s", h.URL, resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, fmt.Errorf("fetch %s: read body: %w", h.URL, err)
	}
	return body, nil
}
