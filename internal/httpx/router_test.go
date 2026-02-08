package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sagerenn/mdict/internal/dict"
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
	return setupRouterWithBasePath(t, "")
}

func setupRouterWithBasePath(t *testing.T, basePath string) http.Handler {
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
	return NewRouter(svc, log, basePath)
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

func TestResourceNameFromPath(t *testing.T) {
	setURLBasePathForTest(t, "")

	tests := []struct {
		name string
		url  string
		want string
	}{
		{name: "no prefix", url: "/lookup?q=a", want: ""},
		{name: "empty", url: "/resource/", want: ""},
		{name: "flat name", url: "/resource/main.css?dict=d1", want: "main.css"},
		{name: "subpath", url: "/resource/js/app.js?dict=d1", want: "js/app.js"},
		{name: "encoded slash in segment", url: "/resource/image%2Ficons/logo.svg?dict=d1", want: "image/icons/logo.svg"},
		{name: "space and unicode", url: "/resource/audio/%E8%AF%8D%20%E5%85%B8.mp3?dict=d1", want: "audio/词 典.mp3"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.url, nil)
			got := resourceNameFromPath(req)
			if got != tc.want {
				t.Fatalf("resourceNameFromPath() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestResourceNameFromPathWithBasePath(t *testing.T) {
	setURLBasePathForTest(t, "/dict")

	req := httptest.NewRequest(http.MethodGet, "/dict/resource/js/app.js?dict=d1", nil)
	got := resourceNameFromPath(req)
	if got != "js/app.js" {
		t.Fatalf("resourceNameFromPath() = %q, want %q", got, "js/app.js")
	}
}

func TestLookupWithBasePath(t *testing.T) {
	r := setupRouterWithBasePath(t, "/dict")
	req := httptest.NewRequest(http.MethodGet, "/dict/lookup?q=hello", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func setURLBasePathForTest(t *testing.T, basePath string) {
	t.Helper()
	prev := dict.URLBasePath()
	dict.SetURLBasePath(basePath)
	t.Cleanup(func() {
		dict.SetURLBasePath(prev)
	})
}
