package dict

type Entry struct {
	Word       string `json:"word"`
	Definition string `json:"definition"`
}

type Dictionary interface {
	ID() string
	Name() string
	Lookup(word string) []Entry
	Prefix(prefix string, limit int) []Entry
	Search(query string, limit int) []Entry
}

// ResourceProvider exposes binary resources referenced by dictionary entries.
// Implementations should return ok=false when the resource is missing.
type ResourceProvider interface {
	Resource(name string) (data []byte, contentType string, ok bool)
}
