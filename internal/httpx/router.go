package httpx

import (
	"encoding/json"
	"expvar"
	"html"
	"net/http"
	"strings"
	"time"

	"github.com/sagerenn/mdict/internal/observability"
	"github.com/sagerenn/mdict/internal/service"
)

type Router struct {
	svc *service.Service
}

type healthResponse struct {
	Status string    `json:"status"`
	Time   time.Time `json:"time"`
}

type dictResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type lookupResponse struct {
	Query   string                  `json:"query"`
	Results []service.ResultEntries `json:"results"`
	Count   int                     `json:"count"`
}

type wordsResponse struct {
	Query   string                `json:"query"`
	Results []service.ResultWords `json:"results"`
	Count   int                   `json:"count"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func NewRouter(svc *service.Service, log *observability.Logger) http.Handler {
	r := &Router{svc: svc}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", r.handleHealth)
	mux.HandleFunc("/dicts", r.handleDicts)
	mux.HandleFunc("/lookup", r.handleLookup)
	mux.HandleFunc("/prefix", r.handlePrefix)
	mux.HandleFunc("/search", r.handleSearch)
	mux.HandleFunc("/entry", r.handleEntry)
	mux.HandleFunc("/resource", r.handleResource)
	mux.Handle("/debug/vars", expvar.Handler())

	h := observability.RequestIDMiddleware(mux)
	h = observability.RecoveryMiddleware(log)(h)
	h = observability.LoggingMiddleware(log)(h)
	return h
}

func (r *Router) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{Status: "ok", Time: time.Now().UTC()})
}

func (r *Router) handleDicts(w http.ResponseWriter, _ *http.Request) {
	dicts := r.svc.List()
	resp := make([]dictResponse, 0, len(dicts))
	for _, d := range dicts {
		resp = append(resp, dictResponse{ID: d.ID(), Name: d.Name()})
	}
	writeJSON(w, http.StatusOK, resp)
}

func (r *Router) handleLookup(w http.ResponseWriter, req *http.Request) {
	query := strings.TrimSpace(req.URL.Query().Get("q"))
	if query == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "missing q"})
		return
	}
	limit := observability.ParseLimit(req.URL.Query().Get("limit"), 20)
	dictIDs := splitIDs(req.URL.Query().Get("dict"))
	results := r.svc.Lookup(query, dictIDs, limit)
	resp := lookupResponse{Query: query, Results: results, Count: len(results)}
	writeJSON(w, http.StatusOK, resp)
}

func (r *Router) handlePrefix(w http.ResponseWriter, req *http.Request) {
	query := strings.TrimSpace(req.URL.Query().Get("q"))
	if query == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "missing q"})
		return
	}
	limit := observability.ParseLimit(req.URL.Query().Get("limit"), 20)
	dictIDs := splitIDs(req.URL.Query().Get("dict"))
	results := r.svc.Prefix(query, dictIDs, limit)
	resp := wordsResponse{Query: query, Results: results, Count: len(results)}
	writeJSON(w, http.StatusOK, resp)
}

func (r *Router) handleSearch(w http.ResponseWriter, req *http.Request) {
	query := strings.TrimSpace(req.URL.Query().Get("q"))
	if query == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "missing q"})
		return
	}
	limit := observability.ParseLimit(req.URL.Query().Get("limit"), 20)
	dictIDs := splitIDs(req.URL.Query().Get("dict"))
	results := r.svc.Search(query, dictIDs, limit)
	resp := wordsResponse{Query: query, Results: results, Count: len(results)}
	writeJSON(w, http.StatusOK, resp)
}

func (r *Router) handleEntry(w http.ResponseWriter, req *http.Request) {
	query := strings.TrimSpace(req.URL.Query().Get("q"))
	if query == "" {
		http.Error(w, "missing q", http.StatusBadRequest)
		return
	}
	limit := observability.ParseLimit(req.URL.Query().Get("limit"), 20)
	dictIDs := splitIDs(req.URL.Query().Get("dict"))
	results := r.svc.Lookup(query, dictIDs, limit)

	var b strings.Builder
	b.WriteString("<!doctype html><html><head><meta charset=\"utf-8\"><title>")
	b.WriteString(html.EscapeString(query))
	b.WriteString("</title></head><body>")
	if len(results) == 0 {
		b.WriteString("<p>No results</p>")
	} else {
		for _, res := range results {
			b.WriteString("<section class=\"dict\" data-id=\"")
			b.WriteString(html.EscapeString(res.DictID))
			b.WriteString("\">")
			b.WriteString("<h2>")
			b.WriteString(html.EscapeString(res.DictName))
			b.WriteString("</h2>")
			for _, entry := range res.Entries {
				b.WriteString("<div class=\"entry\">")
				if entry.Word != "" {
					b.WriteString("<h3>")
					b.WriteString(html.EscapeString(entry.Word))
					b.WriteString("</h3>")
				}
				b.WriteString(entry.Definition)
				b.WriteString("</div>")
			}
			b.WriteString("</section>")
		}
	}
	b.WriteString("</body></html>")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(b.String()))
}

func (r *Router) handleResource(w http.ResponseWriter, req *http.Request) {
	dictID := strings.TrimSpace(req.URL.Query().Get("dict"))
	name := strings.TrimSpace(req.URL.Query().Get("name"))
	if dictID == "" || name == "" {
		http.Error(w, "missing dict or name", http.StatusBadRequest)
		return
	}
	data, contentType, ok := r.svc.Resource(dictID, name)
	if !ok {
		http.Error(w, "resource not found", http.StatusNotFound)
		return
	}
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	_ = enc.Encode(payload)
}

func splitIDs(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
