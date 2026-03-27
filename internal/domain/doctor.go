package domain

// DefaultEndpointName is used when an endpoint name is empty or unresolvable.
const DefaultEndpointName = "-"

// DoctorIssue represents a single health check finding.
type DoctorIssue struct {
	Endpoint string `json:"endpoint"`
	Message  string `json:"message"`
	Severity string `json:"severity"` // "error", "warn", "fixed", "ok"
	Hint     string `json:"hint,omitempty"`
}

// DaemonHealthStatus holds daemon-related health info.
type DaemonHealthStatus struct {
	Checked bool `json:"checked"`
	Running bool `json:"running"`
	PID     int  `json:"pid,omitempty"`
}

// EndpointHealth holds health info for a single endpoint.
type EndpointHealth struct {
	Repo     string   `json:"repo"`
	Dir      string   `json:"dir"`
	Produces []string `json:"produces,omitempty"`
	Consumes []string `json:"consumes,omitempty"`
	OK       bool     `json:"ok"`
}

// DoctorReport holds the complete health check result.
type DoctorReport struct {
	Healthy      bool               `json:"healthy"`
	Issues       []DoctorIssue      `json:"issues"`
	Endpoints    []EndpointHealth   `json:"endpoints,omitempty"`
	DaemonStatus DaemonHealthStatus `json:"daemon_status"`
}

// AddError appends an error-level issue and marks the report unhealthy.
func (r *DoctorReport) AddError(endpoint, msg string) {
	r.Healthy = false
	r.Issues = append(r.Issues, DoctorIssue{Endpoint: endpoint, Message: msg, Severity: "error"})
}

// AddWarn appends a warning-level issue.
func (r *DoctorReport) AddWarn(endpoint, msg string) {
	r.Issues = append(r.Issues, DoctorIssue{Endpoint: endpoint, Message: msg, Severity: "warn"})
}

// AddErrorWithHint appends an error-level issue with a remediation hint.
func (r *DoctorReport) AddErrorWithHint(endpoint, msg, hint string) {
	r.Healthy = false
	r.Issues = append(r.Issues, DoctorIssue{Endpoint: endpoint, Message: msg, Severity: "error", Hint: hint})
}

// AddWarnWithHint appends a warning-level issue with a remediation hint.
func (r *DoctorReport) AddWarnWithHint(endpoint, msg, hint string) {
	r.Issues = append(r.Issues, DoctorIssue{Endpoint: endpoint, Message: msg, Severity: "warn", Hint: hint})
}

// AddFixed appends a fixed-level issue (auto-repaired).
func (r *DoctorReport) AddFixed(endpoint, msg string) {
	r.Issues = append(r.Issues, DoctorIssue{Endpoint: endpoint, Message: msg, Severity: "fixed"})
}

// AddOK appends an ok-level issue (informational).
func (r *DoctorReport) AddOK(endpoint, msg string) {
	r.Issues = append(r.Issues, DoctorIssue{Endpoint: endpoint, Message: msg, Severity: "ok"})
}
