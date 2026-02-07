package filedict

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
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

func NewFromTSV(id, name, path, delimiter string, caseFold bool) (*Dictionary, error) {
	if id == "" || name == "" {
		return nil, errors.New("id and name are required")
	}
	if delimiter == "" {
		delimiter = "\t"
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
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, delimiter)
		if len(parts) < 2 {
			continue
		}
		word := strings.TrimSpace(parts[0])
		def := strings.TrimSpace(strings.Join(parts[1:], delimiter))
		if word == "" || def == "" {
			continue
		}
		key := normalize(word, caseFold)
		idx[key] = append(idx[key], def)
		if _, ok := orig[key]; !ok {
			orig[key] = word
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

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

func NewFromJSON(id, name, path string, caseFold bool) (*Dictionary, error) {
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
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var entries []dict.Entry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	idx := make(map[string][]string)
	orig := make(map[string]string)
	for _, e := range entries {
		word := strings.TrimSpace(e.Word)
		def := strings.TrimSpace(e.Definition)
		if word == "" || def == "" {
			continue
		}
		key := normalize(word, caseFold)
		idx[key] = append(idx[key], def)
		if _, ok := orig[key]; !ok {
			orig[key] = word
		}
	}
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

func Load(id, name, path, typ, delimiter string, caseFold bool) (*Dictionary, error) {
	switch strings.ToLower(typ) {
	case "tsv", "tab", "txt":
		return NewFromTSV(id, name, path, delimiter, caseFold)
	case "json":
		return NewFromJSON(id, name, path, caseFold)
	case "":
		// Attempt by extension.
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".json" {
			return NewFromJSON(id, name, path, caseFold)
		}
		return NewFromTSV(id, name, path, delimiter, caseFold)
	default:
		return nil, errors.New("unsupported dictionary type: " + typ)
	}
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
