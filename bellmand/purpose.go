package main

import "regexp"

var purposePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

func validPurpose(purpose string) bool {
	return purpose == "" || purposePattern.MatchString(purpose)
}

func warnInvalidPurpose(purpose, apiKeyId, keyName string) {
	if validPurpose(purpose) {
		return
	}
	logger.Warn("purpose does not match pattern",
		"apiKeyId", apiKeyId,
		"key", keyName,
		"purpose", purpose,
		"pattern", purposePattern.String(),
	)
}
