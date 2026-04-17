package domain

// DefaultEndpointName is used when an endpoint name is empty or unresolvable.
const DefaultEndpointName = "-"

// CheckStatus represents the outcome of a single doctor check.
type CheckStatus int

const (
	CheckOK CheckStatus = iota
	CheckFail
	CheckSkip
	CheckWarn
	CheckFixed
)

// StatusLabel returns a display string for the check status.
func (s CheckStatus) StatusLabel() string {
	switch s {
	case CheckOK:
		return "OK"
	case CheckFail:
		return "FAIL"
	case CheckSkip:
		return "SKIP"
	case CheckWarn:
		return "WARN"
	case CheckFixed:
		return "FIX"
	default:
		return "????"
	}
}

// DoctorCheck holds the outcome of a single doctor check.
type DoctorCheck struct {
	Name    string
	Status  CheckStatus
	Message string
	Hint    string // optional remediation hint shown on failure
}

// DaemonHealthStatus holds daemon-related health info.
type DaemonHealthStatus struct {
	Checked bool `json:"checked"`
	Running bool `json:"running"`
	PID     int  `json:"pid,omitempty"`
}

// EndpointHealth holds health info for a single endpoint.
type EndpointHealth struct { // nosemgrep: first-class-collection.raw-slice-field-domain-go -- JSON wire-format DTO for doctor output; custom marshal would break JSON compat [permanent]
	Repo     string   `json:"repo"`
	Dir      string   `json:"dir"`
	Produces []string `json:"produces,omitempty"`
	Consumes []string `json:"consumes,omitempty"`
	OK       bool     `json:"ok"`
}

// DoctorReport holds the complete health check result.
type DoctorReport struct { // nosemgrep: first-class-collection.raw-slice-field-domain-go -- read-mostly result view; wrapping would require 12+ call-site migration with minimal safety benefit [permanent]
	Healthy      bool
	Checks       []DoctorCheck
	Endpoints    []EndpointHealth
	DaemonStatus DaemonHealthStatus
}

// AddError appends a FAIL check and marks the report unhealthy.
func (r *DoctorReport) AddError(name, msg string) {
	r.Healthy = false
	r.Checks = append(r.Checks, DoctorCheck{Name: name, Status: CheckFail, Message: msg})
}

// AddWarn appends a WARN check.
func (r *DoctorReport) AddWarn(name, msg string) {
	r.Checks = append(r.Checks, DoctorCheck{Name: name, Status: CheckWarn, Message: msg})
}

// AddErrorWithHint appends a FAIL check with a remediation hint.
func (r *DoctorReport) AddErrorWithHint(name, msg, hint string) {
	r.Healthy = false
	r.Checks = append(r.Checks, DoctorCheck{Name: name, Status: CheckFail, Message: msg, Hint: hint})
}

// AddWarnWithHint appends a WARN check with a remediation hint.
func (r *DoctorReport) AddWarnWithHint(name, msg, hint string) {
	r.Checks = append(r.Checks, DoctorCheck{Name: name, Status: CheckWarn, Message: msg, Hint: hint})
}

// AddFixed appends a FIX check (auto-repaired).
func (r *DoctorReport) AddFixed(name, msg string) {
	r.Checks = append(r.Checks, DoctorCheck{Name: name, Status: CheckFixed, Message: msg})
}

// AddOK appends an OK check (informational).
func (r *DoctorReport) AddOK(name, msg string) {
	r.Checks = append(r.Checks, DoctorCheck{Name: name, Status: CheckOK, Message: msg})
}
