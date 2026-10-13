package fetch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchOK(t *testing.T) {
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.UserAgent()
		w.Write([]byte("<html>menu</html>"))
	}))
	defer srv.Close()

	body, err := New(srv.URL).Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "<html>menu</html>" {
		t.Errorf("body = %q", body)
	}
	if !strings.HasPrefix(gotUA, "menubot/") {
		t.Errorf("User-Agent = %q", gotUA)
	}
}

func TestFetchNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	_, err := New(srv.URL).Fetch(context.Background())
	if err == nil || !strings.Contains(err.Error(), "503") {
		t.Errorf("err = %v, want status 503 error", err)
	}
}
