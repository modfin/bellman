package main

import (
	"strings"
	"testing"
)

func TestValidPurpose(t *testing.T) {
	valid := []string{"", "a", "report.summarize", "batch_job-2", "0abc", strings.Repeat("a", 64)}
	for _, p := range valid {
		if !validPurpose(p) {
			t.Errorf("validPurpose(%q) = false, want true", p)
		}
	}
	invalid := []string{"Report", ".report", "-report", "report summarize", "report/summarize", "ö", strings.Repeat("a", 65)}
	for _, p := range invalid {
		if validPurpose(p) {
			t.Errorf("validPurpose(%q) = true, want false", p)
		}
	}
}
