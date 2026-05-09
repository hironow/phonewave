package session_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hironow/phonewave/internal/domain"
	"github.com/hironow/phonewave/internal/session"
)

// relay-preserve invariant tests (ADR 0005)
//
// phonewave is a relay-only courier daemon: it reads a D-Mail emitted by another
// tool's outbox and delivers the raw bytes byte-for-byte to the target inbox.
// metadata.requester_actor_type / requester_actor_source / initiating_actor_type
// are injected by producers (sightjack/paintress/amadeus per ADR 0017) and MUST
// pass through phonewave unchanged so the gateway-side 4-eyes flow can classify
// the requester correctly.
//
// These tests pin the invariant: phonewave never overwrites, deletes, or
// re-marshals actor-type metadata while relaying.

const dmailWithActorTypeAIAgent = `---
dmail-schema-version: "1"
name: spec-actortype-ai-agent_00000000
kind: specification
description: "with ai-agent actor type"
metadata:
  requester_actor_type: ai-agent
  requester_actor_source: env
---

# Spec body
`

const dmailWithActorTypeHumanOperator = `---
dmail-schema-version: "1"
name: spec-actortype-human_00000000
kind: specification
description: "with human-operator actor type"
metadata:
  requester_actor_type: human-operator
  requester_actor_source: env
---

# Spec body
`

const dmailWithDaemonInitiating = `---
dmail-schema-version: "1"
name: spec-actortype-daemon_00000000
kind: specification
description: "daemon with initiating"
metadata:
  requester_actor_type: workspace-daemon
  requester_actor_source: env
  initiating_actor_type: human-operator
---

# Spec body
`

const dmailLegacyNoActorType = `---
dmail-schema-version: "1"
name: spec-legacy-noactor_00000000
kind: specification
description: "legacy compat, no actor type"
---

# Spec body
`

func setupRelayFixture(t *testing.T, dmailContent string, dmailName string) (dmailPath string, deliveredPath string, routes []domain.ResolvedRoute) {
	t.Helper()
	repoDir := t.TempDir()
	outbox := filepath.Join(repoDir, ".siren", "outbox")
	inbox := filepath.Join(repoDir, ".expedition", "inbox")
	if err := os.MkdirAll(outbox, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(inbox, 0o755); err != nil {
		t.Fatal(err)
	}
	dmailPath = filepath.Join(outbox, dmailName+".md")
	if err := os.WriteFile(dmailPath, []byte(dmailContent), 0o644); err != nil {
		t.Fatal(err)
	}
	routes = []domain.ResolvedRoute{
		{Kind: "specification", FromOutbox: outbox, ToInboxes: []string{inbox}},
	}
	deliveredPath = filepath.Join(inbox, dmailName+".md")
	return dmailPath, deliveredPath, routes
}

func relayAndReadDelivered(t *testing.T, dmailContent, dmailName string) []byte {
	t.Helper()
	dmailPath, deliveredPath, routes := setupRelayFixture(t, dmailContent, dmailName)
	ds := newTestDeliveryStore(t)
	if _, err := session.Deliver(context.Background(), dmailPath, routes, ds); err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	delivered, err := os.ReadFile(deliveredPath)
	if err != nil {
		t.Fatalf("read delivered: %v", err)
	}
	return delivered
}

func TestDeliverData_PreservesActorType_HumanOperator(t *testing.T) {
	// given — producer-emitted D-Mail with requester_actor_type=human-operator
	delivered := relayAndReadDelivered(t, dmailWithActorTypeHumanOperator, "spec-actortype-human_00000000")

	// then — actor type keys preserved bit-perfect
	parsed, err := domain.ParseDMailFrontmatter(delivered)
	if err != nil {
		t.Fatalf("parse delivered: %v", err)
	}
	if got := parsed.Metadata["requester_actor_type"]; got != "human-operator" {
		t.Errorf("requester_actor_type: got %q, want human-operator", got)
	}
	if got := parsed.Metadata["requester_actor_source"]; got != "env" {
		t.Errorf("requester_actor_source: got %q, want env", got)
	}
}

func TestDeliverData_PreservesActorType_AIAgent(t *testing.T) {
	// given — producer-emitted D-Mail with requester_actor_type=ai-agent
	delivered := relayAndReadDelivered(t, dmailWithActorTypeAIAgent, "spec-actortype-ai-agent_00000000")

	// then
	parsed, err := domain.ParseDMailFrontmatter(delivered)
	if err != nil {
		t.Fatalf("parse delivered: %v", err)
	}
	if got := parsed.Metadata["requester_actor_type"]; got != "ai-agent" {
		t.Errorf("requester_actor_type: got %q, want ai-agent", got)
	}
}

func TestDeliverData_PreservesActorType_DaemonWithInitiating(t *testing.T) {
	// given — daemon-emitted D-Mail carrying initiating actor (ADR 0036 effective rule)
	delivered := relayAndReadDelivered(t, dmailWithDaemonInitiating, "spec-actortype-daemon_00000000")

	// then — both keys preserved
	parsed, err := domain.ParseDMailFrontmatter(delivered)
	if err != nil {
		t.Fatalf("parse delivered: %v", err)
	}
	if got := parsed.Metadata["requester_actor_type"]; got != "workspace-daemon" {
		t.Errorf("requester_actor_type: got %q, want workspace-daemon", got)
	}
	if got := parsed.Metadata["initiating_actor_type"]; got != "human-operator" {
		t.Errorf("initiating_actor_type: got %q, want human-operator", got)
	}
}

func TestDeliverData_PreservesActorSource(t *testing.T) {
	// given — producer-emitted D-Mail with explicit requester_actor_source=env.
	// phonewave MUST NOT rewrite source to "broker" (ADR 0037 Axis 1: producer-side
	// "broker" is forbidden; gateway-only attestation).
	delivered := relayAndReadDelivered(t, dmailWithActorTypeAIAgent, "spec-actortype-ai-agent_00000000")

	// then
	parsed, err := domain.ParseDMailFrontmatter(delivered)
	if err != nil {
		t.Fatalf("parse delivered: %v", err)
	}
	if got := parsed.Metadata["requester_actor_source"]; got != "env" {
		t.Errorf("requester_actor_source: got %q, want env (relay must not rewrite source)", got)
	}
}

func TestDeliverData_NoActorType_LegacyCompat(t *testing.T) {
	// given — legacy D-Mail without actor type metadata.
	// phonewave MUST NOT inject placeholder values during relay.
	delivered := relayAndReadDelivered(t, dmailLegacyNoActorType, "spec-legacy-noactor_00000000")

	// then — no actor type keys must be added
	parsed, err := domain.ParseDMailFrontmatter(delivered)
	if err != nil {
		t.Fatalf("parse delivered: %v", err)
	}
	if v, ok := parsed.Metadata["requester_actor_type"]; ok {
		t.Errorf("requester_actor_type must be absent in legacy compat path, got %q", v)
	}
	if v, ok := parsed.Metadata["requester_actor_source"]; ok {
		t.Errorf("requester_actor_source must be absent in legacy compat path, got %q", v)
	}
	if v, ok := parsed.Metadata["initiating_actor_type"]; ok {
		t.Errorf("initiating_actor_type must be absent in legacy compat path, got %q", v)
	}
}

func TestDeliverData_ByteIdentity(t *testing.T) {
	// The strongest form of relay-preserve: the bytes written to target inbox
	// equal the bytes read from outbox, byte-for-byte. This guards against
	// canonicalization, re-marshal, encoding drift, or any implicit rewrite
	// of the D-Mail body or frontmatter during relay.
	cases := []struct {
		name    string
		content string
		dmail   string
	}{
		{"with-ai-agent-actortype", dmailWithActorTypeAIAgent, "spec-actortype-ai-agent_00000000"},
		{"with-human-actortype", dmailWithActorTypeHumanOperator, "spec-actortype-human_00000000"},
		{"with-daemon-initiating", dmailWithDaemonInitiating, "spec-actortype-daemon_00000000"},
		{"legacy-no-actortype", dmailLegacyNoActorType, "spec-legacy-noactor_00000000"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			delivered := relayAndReadDelivered(t, tc.content, tc.dmail)
			if !bytes.Equal(delivered, []byte(tc.content)) {
				t.Errorf("byte-identity violated:\nwant=%q\n got=%q", tc.content, string(delivered))
			}
		})
	}
}
