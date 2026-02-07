package service

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sagerenn/mdict/internal/cache"
	"github.com/sagerenn/mdict/internal/dict"
	"github.com/sagerenn/mdict/internal/dict/registry"
)

type Service struct {
	reg   *registry.Registry
	cache *cache.Cache
}

type ResultEntries struct {
	DictID   string       `json:"dict_id"`
	DictName string       `json:"dict_name"`
	Entries  []dict.Entry `json:"entries"`
}

type ResultWords struct {
	DictID   string   `json:"dict_id"`
	DictName string   `json:"dict_name"`
	Words    []string `json:"words"`
}

func New(reg *registry.Registry) *Service {
	return &Service{
		reg:   reg,
		cache: cache.New(1024, 5*time.Minute),
	}
}

func (s *Service) Lookup(word string, dictIDs []string, limit int) []ResultEntries {
	if limit <= 0 {
		limit = 20
	}
	word = strings.TrimSpace(word)
	if word == "" {
		return nil
	}
	cacheKey := makeKey("lookup", word, dictIDs, limit)
	if v, ok := s.cache.Get(cacheKey); ok {
		if res, ok := v.([]ResultEntries); ok {
			return res
		}
	}
	dicts := s.resolveDicts(dictIDs)
	results := make([]ResultEntries, 0, len(dicts))
	for _, d := range dicts {
		entries := d.Lookup(word)
		if len(entries) > limit {
			entries = entries[:limit]
		}
		if len(entries) == 0 {
			continue
		}
		results = append(results, ResultEntries{
			DictID:   d.ID(),
			DictName: d.Name(),
			Entries:  entries,
		})
	}
	s.cache.Set(cacheKey, results)
	return results
}

func (s *Service) Prefix(prefix string, dictIDs []string, limit int) []ResultWords {
	if limit <= 0 {
		limit = 20
	}
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return nil
	}
	cacheKey := makeKey("prefix", prefix, dictIDs, limit)
	if v, ok := s.cache.Get(cacheKey); ok {
		if res, ok := v.([]ResultWords); ok {
			return res
		}
	}
	dicts := s.resolveDicts(dictIDs)
	results := make([]ResultWords, 0, len(dicts))
	for _, d := range dicts {
		entries := d.Prefix(prefix, limit)
		if len(entries) == 0 {
			continue
		}
		words := make([]string, 0, len(entries))
		for _, e := range entries {
			words = append(words, e.Word)
		}
		results = append(results, ResultWords{
			DictID:   d.ID(),
			DictName: d.Name(),
			Words:    words,
		})
	}
	s.cache.Set(cacheKey, results)
	return results
}

func (s *Service) Search(query string, dictIDs []string, limit int) []ResultWords {
	if limit <= 0 {
		limit = 20
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil
	}
	cacheKey := makeKey("search", query, dictIDs, limit)
	if v, ok := s.cache.Get(cacheKey); ok {
		if res, ok := v.([]ResultWords); ok {
			return res
		}
	}
	dicts := s.resolveDicts(dictIDs)
	results := make([]ResultWords, 0, len(dicts))
	for _, d := range dicts {
		entries := d.Search(query, limit)
		if len(entries) == 0 {
			continue
		}
		words := make([]string, 0, len(entries))
		for _, e := range entries {
			words = append(words, e.Word)
		}
		results = append(results, ResultWords{
			DictID:   d.ID(),
			DictName: d.Name(),
			Words:    words,
		})
	}
	s.cache.Set(cacheKey, results)
	return results
}

func (s *Service) resolveDicts(ids []string) []dict.Dictionary {
	if len(ids) == 0 {
		return s.reg.List()
	}
	out := make([]dict.Dictionary, 0, len(ids))
	for _, id := range ids {
		if d, ok := s.reg.Get(id); ok {
			out = append(out, d)
		}
	}
	return out
}

func (s *Service) List() []dict.Dictionary {
	return s.reg.List()
}

func (s *Service) Resource(dictID, name string) ([]byte, string, bool) {
	if dictID == "" || name == "" {
		return nil, "", false
	}
	d, ok := s.reg.Get(dictID)
	if !ok {
		return nil, "", false
	}
	if rp, ok := d.(dict.ResourceProvider); ok {
		return rp.Resource(name)
	}
	return nil, "", false
}

func makeKey(op, q string, dictIDs []string, limit int) string {
	if len(dictIDs) > 0 {
		ids := make([]string, 0, len(dictIDs))
		for _, id := range dictIDs {
			if id != "" {
				ids = append(ids, id)
			}
		}
		sort.Strings(ids)
		return op + "|" + q + "|" + strings.Join(ids, ",") + "|" + strconv.Itoa(limit)
	}
	return op + "|" + q + "||" + strconv.Itoa(limit)
}
