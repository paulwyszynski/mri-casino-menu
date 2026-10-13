// Package telegram sends messages through the Telegram Bot API.
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type Client struct {
	BaseURL string
	Token   string
	ChatID  string
	HTTP    *http.Client
}

func New(token, chatID string) *Client {
	return &Client{
		BaseURL: "https://api.telegram.org",
		Token:   token,
		ChatID:  chatID,
		HTTP:    &http.Client{Timeout: 20 * time.Second},
	}
}

type sendRequest struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode"`
}

type apiResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description"`
}

// Send posts text (Telegram HTML) to the configured chat.
func (c *Client) Send(ctx context.Context, text string) error {
	body, err := json.Marshal(sendRequest{ChatID: c.ChatID, Text: text, ParseMode: "HTML"})
	if err != nil {
		return fmt.Errorf("telegram: encode: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.BaseURL+"/bot"+c.Token+"/sendMessage", bytes.NewReader(body))
	if err != nil {
		return errors.New("telegram: cannot build request")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		// *url.Error embeds the URL, which contains the bot token.
		var uerr *url.Error
		if errors.As(err, &uerr) {
			err = uerr.Err
		}
		return fmt.Errorf("telegram: send: %w", err)
	}
	defer resp.Body.Close()

	var r apiResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&r); err != nil {
		return fmt.Errorf("telegram: %s: decode response: %w", resp.Status, err)
	}
	if !r.OK {
		return fmt.Errorf("telegram: %s: %s", resp.Status, r.Description)
	}
	return nil
}
