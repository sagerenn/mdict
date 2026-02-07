package registry

import (
	"errors"
	"sync"

	"github.com/sagerenn/mdict/internal/dict"
)

type Registry struct {
	mu    sync.RWMutex
	byID  map[string]dict.Dictionary
	order []dict.Dictionary
}

func New() *Registry {
	return &Registry{
		byID:  make(map[string]dict.Dictionary),
		order: nil,
	}
}

func (r *Registry) Add(d dict.Dictionary) error {
	if d == nil {
		return errors.New("dictionary is nil")
	}
	id := d.ID()
	if id == "" {
		return errors.New("dictionary id is empty")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.byID[id]; exists {
		return errors.New("duplicate dictionary id: " + id)
	}
	r.byID[id] = d
	r.order = append(r.order, d)
	return nil
}

func (r *Registry) Get(id string) (dict.Dictionary, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.byID[id]
	return d, ok
}

func (r *Registry) List() []dict.Dictionary {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]dict.Dictionary, 0, len(r.order))
	out = append(out, r.order...)
	return out
}

func (r *Registry) MustAddAll(dicts []dict.Dictionary) error {
	for _, d := range dicts {
		if err := r.Add(d); err != nil {
			return err
		}
	}
	return nil
}
