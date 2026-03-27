package domain

// UnifiedCheck is the normalized check format across all 4 TAP tools.
// Each tool's doctor output is converted to this shape for aggregation.
type UnifiedCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // "OK", "FAIL", "WARN", "SKIP", "FIX"
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
}

// ToolSection groups checks by tool.
type ToolSection struct {
	Tool   string         `json:"tool"`
	Path   string         `json:"path,omitempty"` // repo path for non-phonewave tools
	Checks []UnifiedCheck `json:"checks"`
	Error  string         `json:"error,omitempty"` // set if tool binary failed to execute
}

// UnifiedDoctorReport is the aggregated report from `phonewave doctor --all`.
type UnifiedDoctorReport struct {
	Sections  []ToolSection  `json:"sections"`
	CrossTool []UnifiedCheck `json:"cross_tool,omitempty"`
	Healthy   bool           `json:"healthy"`
}

// IsHealthy returns true when all sections have no FAIL checks.
func (r *UnifiedDoctorReport) IsHealthy() bool {
	for _, s := range r.Sections {
		if s.Error != "" {
			return false
		}
		for _, c := range s.Checks {
			if c.Status == "FAIL" {
				return false
			}
		}
	}
	for _, c := range r.CrossTool {
		if c.Status == "FAIL" {
			return false
		}
	}
	return true
}
