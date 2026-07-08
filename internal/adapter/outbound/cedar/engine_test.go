package cedar

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ifundeasy/test-permission/internal/core/domain"
	"github.com/ifundeasy/test-permission/internal/core/port"
)

// seedEntities mirrors db/init.sql so the engine can be tested without Postgres.
func seedEntities() []domain.Entity {
	return []domain.Entity{
		{UID: domain.EntityUID{Type: "User", ID: "alice"}, Parents: []domain.EntityUID{{Type: "Group", ID: "engineering"}}},
		{UID: domain.EntityUID{Type: "User", ID: "bob"}, Parents: []domain.EntityUID{{Type: "Group", ID: "engineering"}}},
		{UID: domain.EntityUID{Type: "User", ID: "carol"}, Parents: []domain.EntityUID{{Type: "Role", ID: "admin"}}},
		{UID: domain.EntityUID{Type: "User", ID: "dave"}},
		{UID: domain.EntityUID{Type: "Group", ID: "engineering"}},
		{UID: domain.EntityUID{Type: "Role", ID: "admin"}},
		{UID: domain.EntityUID{Type: "Folder", ID: "eng"}},
		{
			UID:        domain.EntityUID{Type: "Document", ID: "design"},
			Attributes: map[string]any{"owner": domain.EntityUID{Type: "User", ID: "alice"}, "confidential": false},
			Parents:    []domain.EntityUID{{Type: "Folder", ID: "eng"}},
		},
		{
			UID:        domain.EntityUID{Type: "Document", ID: "secret"},
			Attributes: map[string]any{"owner": domain.EntityUID{Type: "User", ID: "alice"}, "confidential": true},
			Parents:    []domain.EntityUID{{Type: "Folder", ID: "eng"}},
		},
	}
}

func newTestEngine(t *testing.T) port.PolicyEngine {
	t.Helper()
	doc, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "policies", "policy.cedar"))
	if err != nil {
		t.Fatalf("read policy: %v", err)
	}
	e, err := NewEngine("policy.cedar", doc)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	return e
}

// TestDecisionMatrix pins the 8 canonical scenarios from demo/cedar.http.
func TestDecisionMatrix(t *testing.T) {
	e := newTestEngine(t)
	ents := seedEntities()

	cases := []struct {
		name                        string
		principal, action, resource string
		want                        domain.Decision
	}{
		{"alice view design (owner + group)", "alice", "view", "design", domain.Allow},
		{"bob view design (group -> folder -> doc)", "bob", "view", "design", domain.Allow},
		{"bob edit design (group only grants view)", "bob", "edit", "design", domain.Deny},
		{"dave view design (no relationship)", "dave", "view", "design", domain.Deny},
		{"carol view design (admin role)", "carol", "view", "design", domain.Allow},
		{"bob view secret (confidential, not owner)", "bob", "view", "secret", domain.Deny},
		{"carol view secret (forbid overrides admin)", "carol", "view", "secret", domain.Deny},
		{"alice view secret (owner overrides confidential)", "alice", "view", "secret", domain.Allow},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := e.IsAuthorized(
				domain.Request{Principal: tc.principal, Action: tc.action, Resource: tc.resource}, ents)
			if err != nil {
				t.Fatalf("IsAuthorized error: %v", err)
			}
			if got != tc.want {
				t.Errorf("decision = %q, want %q", got, tc.want)
			}
		})
	}
}
