package loader

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/sagerenn/mdict/internal/config"
	"github.com/sagerenn/mdict/internal/dict"
	"github.com/sagerenn/mdict/internal/dict/dsl"
	"github.com/sagerenn/mdict/internal/dict/filedict"
	"github.com/sagerenn/mdict/internal/dict/mdict"
	"github.com/sagerenn/mdict/internal/dict/stardict"
)

type Result struct {
	Dicts []dict.Dictionary
	Errs  []error
}

func LoadAll(cfg config.Config) Result {
	res := Result{
		Dicts: make([]dict.Dictionary, 0, len(cfg.Dictionaries)),
		Errs:  nil,
	}
	for _, d := range cfg.Dictionaries {
		if strings.TrimSpace(d.Path) == "" {
			res.Errs = append(res.Errs, fmt.Errorf("dictionary %q missing path", d.ID))
			continue
		}
		if strings.TrimSpace(d.ID) == "" || strings.TrimSpace(d.Name) == "" {
			res.Errs = append(res.Errs, fmt.Errorf("dictionary entry missing id/name for path %q", d.Path))
			continue
		}
		typ := strings.ToLower(strings.TrimSpace(d.Type))
		if typ == "" {
			typ = detectType(d.Path)
		}
		var (
			loaded dict.Dictionary
			err    error
		)
		switch typ {
		case "tsv", "tab", "txt", "json":
			loaded, err = filedict.Load(d.ID, d.Name, d.Path, typ, d.Delimiter, d.CaseFold)
		case "dsl":
			loaded, err = dsl.Load(d.ID, d.Name, d.Path, d.CaseFold)
		case "stardict", "ifo":
			loaded, err = stardict.Load(d.ID, d.Name, d.Path, d.CaseFold)
		case "mdict", "mdx":
			loaded, err = mdict.Load(d.ID, d.Name, d.Path, d.CaseFold)
		default:
			err = fmt.Errorf("unsupported dictionary type: %q", typ)
		}
		if err != nil {
			res.Errs = append(res.Errs, fmt.Errorf("load %s: %w", d.ID, err))
			continue
		}
		res.Dicts = append(res.Dicts, loaded)
	}
	return res
}

func detectType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".ifo":
		return "stardict"
	case ".mdx":
		return "mdict"
	case ".dsl":
		return "dsl"
	case ".json":
		return "json"
	case ".tsv", ".txt":
		return "tsv"
	default:
		return ""
	}
}
