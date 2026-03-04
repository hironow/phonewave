package domain_test

import (
	"testing"

	"github.com/hironow/phonewave/internal/domain"
)

func TestIsDMailFile(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"report.md", true},
		{"feedback.md", true},
		{"README.md", true},
		{"report.txt", false},
		{"report", false},
		{".phonewave-tmp-12345.md", false},
		{".phonewave-tmp-abc.md", false},
		{"regular.md", true},
		{".hidden.md", true},
		{"", false},
		{"dir/report.md", true},      // basename extraction
		{"path/to/.phonewave-tmp-x.md", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := domain.IsDMailFile(tt.name)
			if got != tt.want {
				t.Errorf("IsDMailFile(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}
