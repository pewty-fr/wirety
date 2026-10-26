package agent

import (
	"testing"
	"time"
)

func TestAppliedPushes(t *testing.T) {
	var p appliedPushes
	if p.unchanged("firewall", "v1", time.Minute) {
		t.Fatal("nothing applied yet, reported unchanged")
	}
	p.applied("firewall", "v1")
	if !p.unchanged("firewall", "v1", time.Minute) {
		t.Error("same value reported changed")
	}
	if p.unchanged("firewall", "v2", time.Minute) {
		t.Error("new value reported unchanged")
	}
	if p.unchanged("dns", "v1", 0) {
		t.Error("other name reported unchanged")
	}

	// Past maxAge, an unchanged value is applied again.
	last := p.last["firewall"]
	last.at = time.Now().Add(-2 * time.Minute)
	p.last["firewall"] = last
	if p.unchanged("firewall", "v1", time.Minute) {
		t.Error("value older than maxAge reported unchanged")
	}
	if !p.unchanged("firewall", "v1", 0) {
		t.Error("no age limit: same value reported changed")
	}
}
