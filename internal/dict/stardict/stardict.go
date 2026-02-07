package stardict

import (
	"html"
	"mime"
	"os"
	"path/filepath"
	"sort"
	"strings"

	gd "github.com/sagerenn/mdict/internal/dict"

	std "github.com/ianlewis/go-stardict"
	"github.com/ianlewis/go-stardict/dict"
	"github.com/ianlewis/go-stardict/idx"
)

type Dictionary struct {
	id          string
	name        string
	caseFold    bool
	sd          *std.Stardict
	dict        *dict.Dict
	entries     []entry
	normMap     map[string][]int
	sortedN     []string
	sortedW     []string
	ifoPath     string
	resourceDir string
}

func Load(id, name, ifoPath string, caseFold bool) (*Dictionary, error) {
	sd, err := std.Open(ifoPath, nil)
	if err != nil {
		return nil, err
	}
	if name == "" {
		name = sd.Bookname()
	}
	d, err := sd.Dict()
	if err != nil {
		return nil, err
	}

	if cached, ok, err := loadCache(ifoPath, caseFold); err == nil && ok {
		base := strings.TrimSuffix(ifoPath, filepath.Ext(ifoPath))
		return &Dictionary{
			id:          id,
			name:        name,
			caseFold:    caseFold,
			sd:          sd,
			dict:        d,
			entries:     cached.Entries,
			normMap:     cached.NormToEntry,
			sortedN:     cached.SortedNorm,
			sortedW:     cached.SortedWord,
			ifoPath:     ifoPath,
			resourceDir: base + ".files",
		}, nil
	}

	sc, err := sd.IndexScanner()
	if err != nil {
		return nil, err
	}
	defer sc.Close()

	entries := make([]entry, 0, 1024)
	normMap := make(map[string][]int)
	type item struct {
		norm string
		word string
	}
	items := make([]item, 0, 1024)

	for sc.Scan() {
		w := sc.Word()
		idxPos := len(entries)
		entries = append(entries, entry{
			Word:   w.Word,
			Offset: w.Offset,
			Size:   w.Size,
		})
		norm := normalize(w.Word, caseFold)
		normMap[norm] = append(normMap[norm], idxPos)
		items = append(items, item{norm: norm, word: w.Word})
	}
	if err := sc.Err(); err != nil {
		return nil, err
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

	_ = saveCache(ifoPath, &cacheIndex{
		CaseFold:    caseFold,
		Sources:     mustSources(ifoPath),
		Entries:     entries,
		NormToEntry: normMap,
		SortedNorm:  sortedN,
		SortedWord:  sortedW,
	})

	return &Dictionary{
		id:          id,
		name:        name,
		caseFold:    caseFold,
		sd:          sd,
		dict:        d,
		entries:     entries,
		normMap:     normMap,
		sortedN:     sortedN,
		sortedW:     sortedW,
		ifoPath:     ifoPath,
		resourceDir: strings.TrimSuffix(ifoPath, filepath.Ext(ifoPath)) + ".files",
	}, nil
}

func (d *Dictionary) ID() string {
	return d.id
}

func (d *Dictionary) Name() string {
	return d.name
}

func (d *Dictionary) Lookup(word string) []gd.Entry {
	norm := normalize(word, d.caseFold)
	idxs := d.normMap[norm]
	if len(idxs) == 0 {
		return nil
	}
	out := make([]gd.Entry, 0, len(idxs))
	for _, i := range idxs {
		e := d.entries[i]
		def, ok := d.readDefinition(e)
		if !ok {
			continue
		}
		out = append(out, gd.Entry{Word: e.Word, Definition: def})
	}
	return out
}

func (d *Dictionary) Prefix(prefix string, limit int) []gd.Entry {
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
	out := make([]gd.Entry, 0, limit)
	for i := idx; i < len(d.sortedN) && len(out) < limit; i++ {
		if !strings.HasPrefix(d.sortedN[i], pfx) {
			break
		}
		out = append(out, gd.Entry{Word: d.sortedW[i]})
	}
	return out
}

func (d *Dictionary) Search(query string, limit int) []gd.Entry {
	if limit <= 0 {
		limit = 20
	}
	q := normalize(query, d.caseFold)
	out := make([]gd.Entry, 0, limit)
	for i, n := range d.sortedN {
		if strings.Contains(n, q) {
			out = append(out, gd.Entry{Word: d.sortedW[i]})
			if len(out) >= limit {
				break
			}
		}
	}
	return out
}

func (d *Dictionary) readDefinition(e entry) (string, bool) {
	word := idx.Word{Word: e.Word, Offset: e.Offset, Size: e.Size}
	w, err := d.dict.Word(&word)
	if err != nil {
		return "", false
	}
	def := dataToHTML(w.Data, d.id)
	if def != "" {
		def = `<div id="gdarticlefrom-` + gd.ScopeID(d.id) + `" class="stardict">` + def + `</div>`
	}
	return def, true
}

func dataToHTML(data []*dict.Data, dictID string) string {
	var b strings.Builder
	for _, d := range data {
		s := strings.TrimSpace(renderData(d, dictID))
		if s == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(s)
	}
	return strings.TrimSpace(b.String())
}

func normalize(s string, caseFold bool) string {
	if caseFold {
		return strings.ToLower(strings.TrimSpace(s))
	}
	return strings.TrimSpace(s)
}

func mustSources(ifoPath string) []sourceSig {
	sigs, err := buildSourceSig(ifoPath)
	if err != nil {
		return nil
	}
	return sigs
}

func (d *Dictionary) Resource(name string) ([]byte, string, bool) {
	if name == "" {
		return nil, "", false
	}
	clean := gd.CleanResourceName(name)
	if clean == "" {
		return nil, "", false
	}
	if d.resourceDir != "" {
		p := filepath.Join(d.resourceDir, filepath.FromSlash(clean))
		if data, err := os.ReadFile(p); err == nil {
			if isCSSFile(clean) {
				css := string(data)
				css = gd.RewriteCSSLinks(css, d.id)
				css = gd.IsolateCSS(css, d.id, "")
				data = []byte(css)
			}
			return data, mime.TypeByExtension(filepath.Ext(p)), true
		}
	}
	return nil, "", false
}

func renderData(d *dict.Data, dictID string) string {
	switch d.Type {
	case dict.HTMLType:
		htmlText := `<div class="sdct_h">` + string(d.Data) + `</div>`
		return gd.RewriteResourceLinks(htmlText, dictID)
	case dict.UTFTextType, dict.LocaleTextType:
		return `<div class="sdct_m">` + preformat(string(d.Data)) + `</div>`
	case dict.PangoTextType:
		return `<div class="sdct_g">` + pangoToHTML(string(d.Data)) + `</div>`
	case dict.PhoneticType:
		return `<div class="sdct_t">` + html.EscapeString(string(d.Data)) + `</div>`
	case dict.YinBiaoOrKataType:
		return `<div class="sdct_y">` + html.EscapeString(string(d.Data)) + `</div>`
	case dict.PowerWordType:
		return powerWordToHTML(string(d.Data))
	case dict.MediaWikiType:
		return `<div class="sdct_w">` + html.EscapeString(string(d.Data)) + `</div>`
	case dict.WordNetType:
		return `<div class="sdct_n">` + html.EscapeString(string(d.Data)) + `</div>`
	case dict.ResourceFileListType:
		return renderResourceList(string(d.Data), dictID)
	case dict.XDXFType:
		return xdxfToHTML(string(d.Data), dictID)
	case dict.WavType:
		return `<div class="sdct_W">(an embedded .wav file)</div>`
	case dict.PictureType:
		return `<div class="sdct_P">(an embedded picture file)</div>`
	default:
		if d.Type >= 'a' && d.Type <= 'z' {
			return `<div class="sdct_unknown">` + html.EscapeString(string(d.Data)) + `</div>`
		}
		return ""
	}
}

func preformat(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = html.EscapeString(s)
	s = strings.ReplaceAll(s, "\n", "<br>")
	return s
}

func renderResourceList(s, dictID string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<div class="sdct_r">`)
	parts := strings.Fields(s)
	for _, p := range parts {
		switch {
		case strings.HasPrefix(p, "img:"):
			name := strings.TrimPrefix(p, "img:")
			b.WriteString(`<img src="` + gd.ResourceURL(dictID, name) + `"/>`)
		case strings.HasPrefix(p, "snd:"):
			name := strings.TrimPrefix(p, "snd:")
			b.WriteString(`<audio controls src="` + gd.ResourceURL(dictID, name) + `"></audio>`)
		case strings.HasPrefix(p, "vdo:"):
			name := strings.TrimPrefix(p, "vdo:")
			b.WriteString(`<video controls src="` + gd.ResourceURL(dictID, name) + `"></video>`)
		case strings.HasPrefix(p, "att:"):
			name := strings.TrimPrefix(p, "att:")
			b.WriteString(`<a download href="` + gd.ResourceURL(dictID, name) + `">` + html.EscapeString(name) + `</a>`)
		default:
			b.WriteString(html.EscapeString(p))
		}
	}
	b.WriteString(`</div>`)
	return b.String()
}
