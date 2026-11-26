package models

import (
	"strings"
	"testing"
)

func TestValidatePurpose(t *testing.T) {
	valid := []string{"", "a", "report.summarize", "batch_job-2", "0abc", strings.Repeat("a", 64)}
	for _, p := range valid {
		if err := ValidatePurpose(p); err != nil {
			t.Errorf("ValidatePurpose(%q): unexpected error %v", p, err)
		}
	}
	invalid := []string{"Report", ".report", "-report", "report summarize", "report/summarize", "ö", strings.Repeat("a", 65)}
	for _, p := range invalid {
		if err := ValidatePurpose(p); err == nil {
			t.Errorf("ValidatePurpose(%q): want error", p)
		}
	}
}
