package domain_test

import (
	"testing"

	"github.com/Charuvarthan-T/veille/internal/domain"
)

func TestContestIdentityKey(t *testing.T) {
	c := domain.Contest{Platform: domain.PlatformCodeforces, ExternalID: "123"}
	if c.IdentityKey() != "codeforces:123" {
		t.Fatalf("got %s", c.IdentityKey())
	}
}
