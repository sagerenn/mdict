package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sagerenn/mdict/internal/dict/filedict"
	"github.com/sagerenn/mdict/internal/dict/registry"
	"github.com/sagerenn/mdict/internal/observability"
	"github.com/sagerenn/mdict/internal/service"
)

type lookupResp struct {
	Query   string `json:"query"`
	Count   int    `json:"count"`
	Results []struct {
		DictID   string `json:"dict_id"`
		DictName string `json:"dict_name"`
		Entries  []struct {
			Word       string `json:"word"`
			Definition string `json:"definition"`
		} `json:"entries"`
	} `json:"results"`
}

func setupRouter(t *testing.T) http.Handler {
	t.Helper()
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.tsv")
	data := "hello\tworld\nfoo\tbar\n"
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}

	d, err := filedict.Load("test", "Test", path, "tsv", "\t", true)
	if err != nil {
		t.Fatal(err)
	}

	reg := registry.New()
	if err := reg.Add(d); err != nil {
		t.Fatal(err)
	}

	svc := service.New(reg)
	log := observability.New("error")
	return NewRouter(svc, log)
}

func TestHealth(t *testing.T) {
	r := setupRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestLookup(t *testing.T) {
	r := setupRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/lookup?q=hello", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var resp lookupResp
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Query != "hello" || resp.Count == 0 {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestPrefix(t *testing.T) {
	r := setupRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/prefix?q=f", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestSearch(t *testing.T) {
	r := setupRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/search?q=o", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestDebugVars(t *testing.T) {
	r := setupRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/debug/vars", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "requests_total") {
		t.Fatalf("expected expvar output, got %s", rr.Body.String())
	}
}
