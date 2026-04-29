package domain

// UnifiedCheck is the normalized check format across all 4 TAP tools.
// Each tool's doctor output is converted to this shape for aggregation.
type UnifiedCheck struct { // nosemgrep: structure.multiple-exported-structs-go -- unified doctor family (UnifiedCheck/ToolSection/UnifiedDoctorReport) is cohesive cross-tool doctor READ MODEL set [permanent]
	Name    string `json:"name"`
	Status  string `json:"status"` // "OK", "FAIL", "WARN", "SKIP", "FIX"
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
}

// ToolSection groups checks by tool.
type ToolSection struct { // nosemgrep: structure.multiple-exported-structs-go,first-class-collection.raw-slice-field-domain-go -- unified doctor family; see UnifiedCheck [permanent]
	Tool   string         `json:"tool"`
	Path   string         `json:"path,omitempty"` // repo path for non-phonewave tools
	Checks []UnifiedCheck `json:"checks"`
	Error  string         `json:"error,omitempty"` // set if tool binary failed to execute
}

// UnifiedDoctorReport is the aggregated report from `phonewave doctor --all`.
type UnifiedDoctorReport struct { // nosemgrep: first-class-collection.raw-slice-field-domain-go -- JSON wire-format READ MODEL for unified doctor output; custom marshal would break JSON compat [permanent]
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
