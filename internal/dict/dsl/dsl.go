package dsl

import (
	"bufio"
	"errors"
	"os"
	"sort"
	"strings"

	"github.com/sagerenn/mdict/internal/dict"
	"github.com/sagerenn/mdict/internal/indexcache"
)

type Dictionary struct {
	id       string
	name     string
	caseFold bool
	index    map[string][]string
	words    []string
	original map[string]string
}

func Load(id, name, path string, caseFold bool) (*Dictionary, error) {
	if id == "" || name == "" {
		return nil, errors.New("id and name are required")
	}
	if idx, ok, err := indexcache.Load(path, caseFold); err == nil && ok {
		return &Dictionary{
			id:       id,
			name:     name,
			caseFold: caseFold,
			index:    idx.Entries,
			words:    idx.Words,
			original: idx.Original,
		}, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	idx := make(map[string][]string)
	orig := make(map[string]string)
	scanner := bufio.NewScanner(file)

	var currentWord string
	var defLines []string
	flush := func() {
		if currentWord == "" {
			defLines = nil
			return
		}
		def := strings.TrimSpace(strings.Join(defLines, "\n"))
		if def == "" {
			defLines = nil
			return
		}
		key := normalize(currentWord, caseFold)
		idx[key] = append(idx[key], def)
		if _, ok := orig[key]; !ok {
			orig[key] = currentWord
		}
		defLines = nil
	}

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") {
			continue
		}
		trimmed := strings.TrimRight(line, "\r\n")
		if trimmed == "" {
			flush()
			currentWord = ""
			continue
		}
		if len(trimmed) > 0 && (trimmed[0] == ' ' || trimmed[0] == '\t') {
			if currentWord == "" {
				continue
			}
			defLines = append(defLines, strings.TrimSpace(trimmed))
			continue
		}
		// New headword
		flush()
		currentWord = strings.TrimSpace(trimmed)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	flush()

	words := make([]string, 0, len(idx))
	for k := range idx {
		words = append(words, k)
	}
	sort.Strings(words)

	_ = indexcache.Save(path, caseFold, words, idx, orig)

	return &Dictionary{
		id:       id,
		name:     name,
		caseFold: caseFold,
		index:    idx,
		words:    words,
		original: orig,
	}, nil
}

func (d *Dictionary) ID() string {
	return d.id
}

func (d *Dictionary) Name() string {
	return d.name
}

func (d *Dictionary) Lookup(word string) []dict.Entry {
	key := normalize(word, d.caseFold)
	defs := d.index[key]
	if len(defs) == 0 {
		return nil
	}
	entries := make([]dict.Entry, 0, len(defs))
	for _, def := range defs {
		entries = append(entries, dict.Entry{Word: d.original[key], Definition: def})
	}
	return entries
}

func (d *Dictionary) Prefix(prefix string, limit int) []dict.Entry {
	if limit <= 0 {
		limit = 20
	}
	pfx := normalize(prefix, d.caseFold)
	idx := sort.Search(len(d.words), func(i int) bool {
		return d.words[i] >= pfx
	})
	if idx == len(d.words) {
		return nil
	}
	res := make([]dict.Entry, 0, limit)
	for i := idx; i < len(d.words) && len(res) < limit; i++ {
		w := d.words[i]
		if !strings.HasPrefix(w, pfx) {
			break
		}
		res = append(res, dict.Entry{Word: d.original[w]})
	}
	return res
}

func (d *Dictionary) Search(query string, limit int) []dict.Entry {
	if limit <= 0 {
		limit = 20
	}
	q := normalize(query, d.caseFold)
	res := make([]dict.Entry, 0, limit)
	for _, w := range d.words {
		if strings.Contains(w, q) {
			res = append(res, dict.Entry{Word: d.original[w]})
			if len(res) >= limit {
				break
			}
		}
	}
	return res
}

func normalize(s string, caseFold bool) string {
	if caseFold {
		return strings.ToLower(strings.TrimSpace(s))
	}
	return strings.TrimSpace(s)
}
