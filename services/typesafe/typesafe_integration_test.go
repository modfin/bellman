package typesafe_test

import (
	"os"
	"testing"

	"github.com/modfin/bellman/services/typesafe"
	"github.com/modfin/bellman/testsuite"
)

func TestTypeSafeIntegration(t *testing.T) {
	key := os.Getenv("TYPESAFE_API_KEY")
	if key == "" {
		t.Skip("TYPESAFE_API_KEY not set")
	}

	client := typesafe.New(key)

	testsuite.RunDecide(t, client, typesafe.DecideModel_jev_latest, testsuite.DecideCapabilities{
		Predicate: true,
		Choice:    true,
		Score:     true,
		JSONState: true,
	})
}
