package mdict

import (
	"encoding/gob"
	"errors"
	"os"
	"path/filepath"
)

const cacheVersion = 1

type cacheIndex struct {
	Version       int
	CaseFold      bool
	SourcePath    string
	SourceSize    int64
	SourceMtime   int64
	Entries       []wordEntry
	NormToEntries map[string][]int
	SortedNorm    []string
	SortedWord    []string
}

type wordEntry struct {
	Word    string
	Offsets []int
}

func cachePath(path string) string {
	return path + ".gdapi.mdx.idx"
}

func loadCache(path string, caseFold bool) (*cacheIndex, bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, false, err
	}
	f, err := os.Open(cachePath(path))
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
	if filepath.Clean(idx.SourcePath) != filepath.Clean(path) || idx.SourceSize != info.Size() || idx.SourceMtime != info.ModTime().UnixNano() {
		return nil, false, nil
	}
	return &idx, true, nil
}

func saveCache(path string, caseFold bool, entries []wordEntry, norm map[string][]int, sortedN, sortedW []string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	idx := cacheIndex{
		Version:       cacheVersion,
		CaseFold:      caseFold,
		SourcePath:    filepath.Clean(path),
		SourceSize:    info.Size(),
		SourceMtime:   info.ModTime().UnixNano(),
		Entries:       entries,
		NormToEntries: norm,
		SortedNorm:    sortedN,
		SortedWord:    sortedW,
	}
	idxPath := cachePath(path)
	tmp, err := os.CreateTemp(filepath.Dir(idxPath), filepath.Base(idxPath)+".tmp.*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	enc := gob.NewEncoder(tmp)
	if err := enc.Encode(&idx); err != nil {
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
