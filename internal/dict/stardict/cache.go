package stardict

import (
	"encoding/gob"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const cacheVersion = 1

type sourceSig struct {
	Path  string
	Size  int64
	Mtime int64
}

type cacheIndex struct {
	Version      int
	CaseFold     bool
	Sources      []sourceSig
	Entries      []entry
	NormToEntry  map[string][]int
	SortedNorm   []string
	SortedWord   []string
	SourceIfopath string
}

type entry struct {
	Word   string
	Offset uint64
	Size   uint32
}

func cachePath(ifoPath string) string {
	return ifoPath + ".gdapi.sdict.idx"
}

func loadCache(ifoPath string, caseFold bool) (*cacheIndex, bool, error) {
	f, err := os.Open(cachePath(ifoPath))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, err
	}
	defer f.Close()

	dec := gob.NewDecoder(f)
	var idx cacheIndex
	if err := dec.Decode(&idx); err != nil {
		return nil, false, err
	}
	if idx.Version != cacheVersion || idx.CaseFold != caseFold {
		return nil, false, nil
	}
	sigs, err := buildSourceSig(ifoPath)
	if err != nil {
		return nil, false, nil
	}
	if !sameSources(idx.Sources, sigs) {
		return nil, false, nil
	}
	return &idx, true, nil
}

func saveCache(ifoPath string, idx *cacheIndex) error {
	idx.Version = cacheVersion
	idx.SourceIfopath = filepath.Clean(ifoPath)
	idxPath := cachePath(ifoPath)
	tmp, err := os.CreateTemp(filepath.Dir(idxPath), filepath.Base(idxPath)+".tmp.*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	enc := gob.NewEncoder(tmp)
	if err := enc.Encode(idx); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, idxPath); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}

func buildSourceSig(ifoPath string) ([]sourceSig, error) {
	paths := []string{ifoPath}
	idxPath, err := findIdxPath(ifoPath)
	if err == nil {
		paths = append(paths, idxPath)
	}
	dictPath, err := findDictPath(ifoPath)
	if err == nil {
		paths = append(paths, dictPath)
	}

	out := make([]sourceSig, 0, len(paths))
	for _, p := range paths {
		clean := filepath.Clean(p)
		info, err := os.Stat(clean)
		if err != nil {
			return nil, err
		}
		out = append(out, sourceSig{Path: clean, Size: info.Size(), Mtime: info.ModTime().UnixNano()})
	}
	return out, nil
}

func sameSources(a, b []sourceSig) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if filepath.Clean(a[i].Path) != filepath.Clean(b[i].Path) {
			return false
		}
		if a[i].Size != b[i].Size || a[i].Mtime != b[i].Mtime {
			return false
		}
	}
	return true
}

func findIdxPath(ifoPath string) (string, error) {
	base := strings.TrimSuffix(ifoPath, filepath.Ext(ifoPath))
	ext := []string{
		".idx",
		".idx.gz",
		".idx.GZ",
		".idx.dz",
		".idx.DZ",
		".IDX",
		".IDX.gz",
		".IDX.GZ",
		".IDX.dz",
		".IDX.DZ",
	}
	for _, e := range ext {
		p := base + e
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", os.ErrNotExist
}

func findDictPath(ifoPath string) (string, error) {
	base := strings.TrimSuffix(ifoPath, filepath.Ext(ifoPath))
	ext := []string{
		".dict",
		".dict.dz",
		".dict.DZ",
		".DICT",
		".DICT.dz",
		".DICT.DZ",
	}
	for _, e := range ext {
		p := base + e
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", os.ErrNotExist
}
