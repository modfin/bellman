package models

import (
	"fmt"
	"regexp"
)

var purposePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

func ValidatePurpose(purpose string) error {
	if purpose == "" || purposePattern.MatchString(purpose) {
		return nil
	}
	return fmt.Errorf("invalid purpose %q, must match %s", purpose, purposePattern.String())
}
