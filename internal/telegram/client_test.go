package telegram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const token = "123456:SECRET-TOKEN"

func TestSendOK(t *testing.T) {
	var gotPath string
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q", r.Header.Get("Content-Type"))
		}
		json.NewDecoder(r.Body).Decode(&got)
		w.Write([]byte(`{"ok":true,"result":{}}`))
	}))
	defer srv.Close()

	c := New(token, "-1001234567890")
	c.BaseURL = srv.URL
	if err := c.Send(context.Background(), "<b>Hallo</b>"); err != nil {
		t.Fatal(err)
	}
	if gotPath != "/bot"+token+"/sendMessage" {
		t.Errorf("path = %q", gotPath)
	}
	if got["chat_id"] != "-1001234567890" || got["text"] != "<b>Hallo</b>" || got["parse_mode"] != "HTML" {
		t.Errorf("payload = %v", got)
	}
}

func TestSendAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"ok":false,"description":"Bad Request: chat not found"}`))
	}))
	defer srv.Close()

	c := New(token, "42")
	c.BaseURL = srv.URL
	err := c.Send(context.Background(), "x")
	if err == nil || !strings.Contains(err.Error(), "chat not found") {
		t.Errorf("err = %v, want API description", err)
	}
}

func TestSendNetworkErrorDoesNotLeakToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	srv.Close() // connection refused from now on

	c := New(token, "42")
	c.BaseURL = srv.URL
	err := c.Send(context.Background(), "x")
	if err == nil {
		t.Fatal("want error")
	}
	if strings.Contains(err.Error(), "SECRET-TOKEN") {
		t.Errorf("error leaks token: %v", err)
	}
}
