// Package catalog loads the application-permission registry: the metadata
// describing the 5 imagined ERP microservices (endpoints, UI pages, UI
// components), every one a first-class permission target.
package catalog

import (
	"encoding/json"
	"fmt"
	"os"
)

type Endpoint struct {
	Key    string `json:"key"`
	Method string `json:"method"`
	Path   string `json:"path"`
}

type Page struct {
	Key        string   `json:"key"`
	Route      string   `json:"route"`
	Components []string `json:"components"`
}

type Service struct {
	Key       string     `json:"key"`
	Name      string     `json:"name"`
	Endpoints []Endpoint `json:"endpoints"`
	Pages     []Page     `json:"pages"`
}

type Catalog struct {
	Services []Service `json:"services"`
}

// Load reads and validates the catalog JSON.
func Load(path string) (*Catalog, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read catalog: %w", err)
	}
	var c Catalog
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("parse catalog: %w", err)
	}
	if len(c.Services) == 0 {
		return nil, fmt.Errorf("catalog has no services")
	}
	return &c, nil
}

// Counts returns (endpoints, pages, components) totals.
func (c *Catalog) Counts() (int, int, int) {
	var eps, pgs, cmps int
	for _, s := range c.Services {
		eps += len(s.Endpoints)
		pgs += len(s.Pages)
		for _, p := range s.Pages {
			cmps += len(p.Components)
		}
	}
	return eps, pgs, cmps
}
