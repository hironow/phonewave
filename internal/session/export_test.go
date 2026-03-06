package session

// white-box-reason: bridge constructor: exposes unexported symbols for external test packages

import (
	"context"
	"os/exec"
)

// NewLocalNotifierForTest creates a LocalNotifier with test overrides.
func NewLocalNotifierForTest(osName string, factory func(ctx context.Context, name string, args ...string) *exec.Cmd) *LocalNotifier {
	return &LocalNotifier{forceOS: osName, cmdFactory: factory}
}

// NewCmdNotifierForTest creates a CmdNotifier with a test command factory.
func NewCmdNotifierForTest(cmdTemplate string, factory func(ctx context.Context, name string, args ...string) *exec.Cmd) *CmdNotifier {
	return &CmdNotifier{cmdTemplate: cmdTemplate, cmdFactory: factory}
}

// ExportSkillsRefAvailable exposes skillsRefAvailable for external tests.
var ExportSkillsRefAvailable = skillsRefAvailable
