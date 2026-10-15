package container

import (
	"regexp"
	"testing"
)

func TestNewID(t *testing.T) {
	a, b := newID(), newID()
	if !regexp.MustCompile(`^[0-9a-f]{12}$`).MatchString(a) {
		t.Errorf("id %q is not twelve lowercase hex characters", a)
	}
	if a == b {
		t.Errorf("two ids collided: %q", a)
	}
}
