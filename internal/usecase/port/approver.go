package port

import "context"

// Approver determines whether an action should proceed.
// Implementations include StdinApprover (human prompt),
// CmdApprover (external command), and AutoApprover (always yes).
type Approver interface {
	RequestApproval(ctx context.Context, message string) (approved bool, err error)
}

// AutoApprover always approves without human interaction.
type AutoApprover struct{}

func (*AutoApprover) RequestApproval(_ context.Context, _ string) (bool, error) {
	return true, nil
}
