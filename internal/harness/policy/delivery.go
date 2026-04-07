package policy

import "github.com/hironow/phonewave/internal/domain"

// SelectDeliveryInboxes applies phonewave's deterministic precedence:
// explicit targets, then explicit target_agent / preferred improvement owner,
// then the original route fan-out.
func SelectDeliveryInboxes(kind domain.DMailKind, inboxes []string, targets []string, metadata domain.CorrectionMetadata) []string {
	selected := append([]string(nil), inboxes...)
	if filtered := domain.FilterInboxesByTargets(selected, targets); len(filtered) > 0 {
		selected = filtered
	}
	if filtered := domain.FilterInboxesByTargetAgent(selected, domain.PreferredImprovementTargetAgent(kind, metadata)); len(filtered) > 0 {
		selected = filtered
	}
	return selected
}
