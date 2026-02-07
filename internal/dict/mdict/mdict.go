package mdict

import (
	"bytes"
	"encoding/binary"
	"log"
	"mime"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode/utf16"

	"github.com/ChaosNyaruko/ondict/decoder"
	"github.com/ChaosNyaruko/ondict/util"
	"github.com/sagerenn/mdict/internal/dict"
)

type Dictionary struct {
	id          string
	name        string
	caseFold    bool
	mdx         *decoder.MDict
	entries     []wordEntry
	normIndex   map[string][]int
	sortedN     []string
	sortedW     []string
	encoding    string
	path        string
	resourceDir string
	resources   []resourceIndex
}

func Load(id, name, path string, caseFold bool) (*Dictionary, error) {
	md := &decoder.MDict{}
	if err := md.Decode(path, false); err != nil {
		return nil, err
	}
	if name == "" {
		name = id
	}

	enc := mdictEncoding(md)
	resources := loadResources(path)
	resDir := filepath.Dir(path)

	if cached, ok, err := loadCache(path, caseFold); err == nil && ok {
		return &Dictionary{
			id:          id,
			name:        name,
			caseFold:    caseFold,
			mdx:         md,
			entries:     cached.Entries,
			normIndex:   cached.NormToEntries,
			sortedN:     cached.SortedNorm,
			sortedW:     cached.SortedWord,
			encoding:    enc,
			path:        path,
			resourceDir: resDir,
			resources:   resources,
		}, nil
	}

	_ = md.Keys() // populate keymap
	keymap := mdictKeyMap(md)

	entries := make([]wordEntry, 0, len(keymap))
	normIndex := make(map[string][]int)
	type item struct {
		norm string
		word string
	}
	items := make([]item, 0, len(keymap))

	for word, offs := range keymap {
		idx := len(entries)
		o := make([]int, 0, len(offs))
		for _, v := range offs {
			o = append(o, int(v))
		}
		entries = append(entries, wordEntry{Word: word, Offsets: o})
		norm := normalize(word, caseFold)
		normIndex[norm] = append(normIndex[norm], idx)
		items = append(items, item{norm: norm, word: word})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].norm == items[j].norm {
			return items[i].word < items[j].word
		}
		return items[i].norm < items[j].norm
	})
	sortedN := make([]string, 0, len(items))
	sortedW := make([]string, 0, len(items))
	for _, it := range items {
		sortedN = append(sortedN, it.norm)
		sortedW = append(sortedW, it.word)
	}

	_ = saveCache(path, caseFold, entries, normIndex, sortedN, sortedW)

	return &Dictionary{
		id:          id,
		name:        name,
		caseFold:    caseFold,
		mdx:         md,
		entries:     entries,
		normIndex:   normIndex,
		sortedN:     sortedN,
		sortedW:     sortedW,
		encoding:    enc,
		path:        path,
		resourceDir: resDir,
		resources:   resources,
	}, nil
}

func (d *Dictionary) ID() string {
	return d.id
}

func (d *Dictionary) Name() string {
	return d.name
}

func (d *Dictionary) Lookup(word string) []dict.Entry {
	return d.lookup(word, make(map[string]bool))
}

func (d *Dictionary) Prefix(prefix string, limit int) []dict.Entry {
	if limit <= 0 {
		limit = 20
	}
	pfx := normalize(prefix, d.caseFold)
	idx := sort.Search(len(d.sortedN), func(i int) bool {
		return d.sortedN[i] >= pfx
	})
	if idx == len(d.sortedN) {
		return nil
	}
	out := make([]dict.Entry, 0, limit)
	for i := idx; i < len(d.sortedN) && len(out) < limit; i++ {
		if !strings.HasPrefix(d.sortedN[i], pfx) {
			break
		}
		out = append(out, dict.Entry{Word: d.sortedW[i]})
	}
	return out
}

func (d *Dictionary) Search(query string, limit int) []dict.Entry {
	if limit <= 0 {
		limit = 20
	}
	q := normalize(query, d.caseFold)
	out := make([]dict.Entry, 0, limit)
	for i, n := range d.sortedN {
		if strings.Contains(n, q) {
			out = append(out, dict.Entry{Word: d.sortedW[i]})
			if len(out) >= limit {
				break
			}
		}
	}
	return out
}

func (d *Dictionary) Resource(name string) ([]byte, string, bool) {
	if name == "" {
		return nil, "", false
	}

	clean := dict.CleanResourceName(name)
	if clean == "" {
		return nil, "", false
	}

	// Try filesystem next to the MDX file first.
	if d.resourceDir != "" {
		p := filepath.Join(d.resourceDir, filepath.FromSlash(clean))
		if data, err := os.ReadFile(p); err == nil {
			if isCSSFile(clean) {
				data = processCSS(data, d.encoding, d.id)
			}
			return data, mime.TypeByExtension(filepath.Ext(p)), true
		}
	}

	for i := range d.resources {
		if data, ok := d.resources[i].read(clean); ok {
			if isCSSFile(clean) {
				data = processCSS(data, d.encoding, d.id)
			}
			return data, mime.TypeByExtension(filepath.Ext(clean)), true
		}
	}

	return nil, "", false
}

func (d *Dictionary) lookup(word string, visited map[string]bool) []dict.Entry {
	q := normalize(word, d.caseFold)
	if visited[q] {
		return nil
	}
	visited[q] = true
	idxs := d.normIndex[q]
	if len(idxs) == 0 {
		return nil
	}
	out := make([]dict.Entry, 0, len(idxs))
	seen := make(map[string]bool)
	for _, i := range idxs {
		entry := d.entries[i]
		for _, off := range entry.Offsets {
			raw := d.decode(d.mdx.ReadAtOffset(off))
			if target := parseRedirect(raw); target != "" {
				redirected := d.lookup(target, visited)
				if len(redirected) > 0 {
					for _, e := range redirected {
						if !seen[e.Word+":"+e.Definition] {
							seen[e.Word+":"+e.Definition] = true
							out = append(out, e)
						}
					}
					continue
				}
			}
			def := strings.TrimSpace(util.ReplaceLINK(raw))
			if def == "" {
				continue
			}
			def = dict.RewriteResourceLinks(def, d.id)
			def = rewriteFontLinksInHTML(def, d.id)
			def = isolateStyleCSSInHTML(def, d.id)
			def = `<div id="gdarticlefrom-` + dict.ScopeID(d.id) + `" class="mdict">` + def + `</div>`
			key := entry.Word + ":" + def
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, dict.Entry{Word: entry.Word, Definition: def})
		}
	}
	return out
}

func (d *Dictionary) decode(b []byte) string {
	if d.encoding == "UTF-16" {
		runes := make([]uint16, len(b)/2)
		_ = binary.Read(bytes.NewBuffer(b), binary.LittleEndian, runes)
		return string(utf16.Decode(runes))
	}
	return string(b)
}

func mdictEncoding(m *decoder.MDict) string {
	v := reflect.ValueOf(m).Elem().FieldByName("encoding")
	if v.IsValid() && v.Kind() == reflect.String {
		return v.String()
	}
	return "UTF-8"
}

func mdictKeyMap(m *decoder.MDict) map[string][]uint64 {
	v := reflect.ValueOf(m).Elem().FieldByName("keymap")
	if !v.IsValid() || v.IsNil() {
		return nil
	}
	out := make(map[string][]uint64)
	for _, k := range v.MapKeys() {
		key := k.String()
		vals := v.MapIndex(k)
		offs := make([]uint64, 0, vals.Len())
		for i := 0; i < vals.Len(); i++ {
			offs = append(offs, uint64(vals.Index(i).Uint()))
		}
		out[key] = offs
	}
	return out
}

func normalize(s string, caseFold bool) string {
	if caseFold {
		return strings.ToLower(strings.TrimSpace(s))
	}
	return strings.TrimSpace(s)
}

type resourceIndex struct {
	dict   *decoder.MDict
	once   sync.Once
	keymap map[string][]uint64
}

func (r *resourceIndex) read(name string) ([]byte, bool) {
	if r == nil || r.dict == nil {
		return nil, false
	}
	r.once.Do(func() {
		_ = r.dict.Keys()
		r.keymap = mdictKeyMap(r.dict)
	})
	if r.keymap == nil {
		return nil, false
	}
	if offs, ok := r.keymap[name]; ok && len(offs) > 0 {
		return r.dict.ReadAtOffset(int(offs[0])), true
	}
	return nil, false
}

func loadResources(mdxPath string) []resourceIndex {
	base := strings.TrimSuffix(mdxPath, filepath.Ext(mdxPath))
	var paths []string
	if _, err := os.Stat(base + ".mdd"); err == nil {
		paths = append(paths, base+".mdd")
	}
	for vol := 1; ; vol++ {
		p := base + "." + strconv.Itoa(vol) + ".mdd"
		if _, err := os.Stat(p); err != nil {
			break
		}
		paths = append(paths, p)
	}
	out := make([]resourceIndex, 0, len(paths))
	for _, p := range paths {
		mdd := &decoder.MDict{}
		if err := mdd.Decode(p, false); err != nil {
			log.Printf("mdict: failed to decode mdd resource file %q: %v", p, err)
			continue
		}
		out = append(out, resourceIndex{dict: mdd})
	}
	return out
}

func parseRedirect(raw string) string {
	if !strings.HasPrefix(raw, "@@@LINK=") {
		return ""
	}
	target := strings.TrimPrefix(raw, "@@@LINK=")
	target = strings.TrimRight(target, "\x00")
	target = strings.TrimSpace(strings.TrimRight(target, "\r\n"))
	return target
}
