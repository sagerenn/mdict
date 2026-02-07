package indexcache

import (
	"encoding/gob"
	"errors"
	"os"
	"path/filepath"
	"time"
)

const currentVersion = 1

type Index struct {
	Version     int
	SourcePath  string
	SourceSize  int64
	SourceMtime int64
	CaseFold    bool

	Words    []string
	Entries  map[string][]string
	Original map[string]string
}

func indexPath(sourcePath string) string {
	return sourcePath + ".gdapi.idx"
}

func Load(sourcePath string, caseFold bool) (*Index, bool, error) {
	info, err := os.Stat(sourcePath)
	if err != nil {
		return nil, false, err
	}
	idxPath := indexPath(sourcePath)
	f, err := os.Open(idxPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, err
	}
	defer f.Close()

	dec := gob.NewDecoder(f)
	var idx Index
	if err := dec.Decode(&idx); err != nil {
		return nil, false, err
	}
	if idx.Version != currentVersion {
		return nil, false, nil
	}
	if idx.CaseFold != caseFold {
		return nil, false, nil
	}
	if idx.SourceSize != info.Size() || idx.SourceMtime != info.ModTime().UnixNano() {
		return nil, false, nil
	}
	if filepath.Clean(idx.SourcePath) != filepath.Clean(sourcePath) {
		return nil, false, nil
	}
	return &idx, true, nil
}

func Save(sourcePath string, caseFold bool, words []string, entries map[string][]string, original map[string]string) error {
	info, err := os.Stat(sourcePath)
	if err != nil {
		return err
	}
	idx := Index{
		Version:     currentVersion,
		SourcePath:  sourcePath,
		SourceSize:  info.Size(),
		SourceMtime: info.ModTime().UnixNano(),
		CaseFold:    caseFold,
		Words:       words,
		Entries:     entries,
		Original:    original,
	}
	idxPath := indexPath(sourcePath)
	tmp := idxPath + "." + time.Now().Format("20060102150405") + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	enc := gob.NewEncoder(f)
	if err := enc.Encode(&idx); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, idxPath)
}
