package services

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// StoredDomain represents a persisted domain configuration.
type StoredDomain struct {
	Domain   string `json:"domain"`
	Upstream string `json:"upstream"`
	Type     string `json:"type"` // "reverse_proxy" or "file_server"
}

// DomainStore persists domain configs to a JSON file so they survive Caddy restarts.
// Thread-safe — safe to call from concurrent HTTP handlers.
type DomainStore struct {
	mu   sync.RWMutex
	path string
}

// NewDomainStore creates a DomainStore backed by the given file path.
func NewDomainStore(dataDir string) *DomainStore {
	return &DomainStore{
		path: filepath.Join(dataDir, "domains.json"),
	}
}

// Load reads all persisted domains from disk.
func (s *DomainStore) Load() ([]StoredDomain, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return []StoredDomain{}, nil
	}
	if err != nil {
		return nil, err
	}
	var domains []StoredDomain
	if err := json.Unmarshal(data, &domains); err != nil {
		return nil, err
	}
	return domains, nil
}

// Add adds or updates a domain entry and saves to disk.
func (s *DomainStore) Add(domain, upstream, handlerType string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	domains, _ := s.load()

	// Replace existing entry for this domain (upsert).
	found := false
	for i, d := range domains {
		if d.Domain == domain {
			domains[i] = StoredDomain{Domain: domain, Upstream: upstream, Type: handlerType}
			found = true
			break
		}
	}
	if !found {
		domains = append(domains, StoredDomain{Domain: domain, Upstream: upstream, Type: handlerType})
	}
	return s.save(domains)
}

// Remove deletes a domain entry by name and saves to disk.
func (s *DomainStore) Remove(domain string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	domains, _ := s.load()
	filtered := domains[:0]
	for _, d := range domains {
		if d.Domain != domain {
			filtered = append(filtered, d)
		}
	}
	return s.save(filtered)
}

// load reads from disk without locking (caller must hold lock).
func (s *DomainStore) load() ([]StoredDomain, error) {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return []StoredDomain{}, nil
	}
	if err != nil {
		return nil, err
	}
	var domains []StoredDomain
	json.Unmarshal(data, &domains)
	return domains, nil
}

// save writes to disk without locking (caller must hold lock).
func (s *DomainStore) save(domains []StoredDomain) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(domains, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0644)
}
