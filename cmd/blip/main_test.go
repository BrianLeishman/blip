package main

import (
	"github.com/BrianLeishman/blip/dashboard"
	"testing"
)

func TestOpenTargetIndependentOfSnapshot(t *testing.T) {
	for _, revision := range []string{"", "old-before-restart", "current"} {
		for _, id := range []string{"ExampleOrg/widget/pull/57", "BrianLeishman/blip/issues/1"} {
			link, ok := openTarget(dashboard.Event{Revision: revision, ID: id})
			if !ok || link != "https://github.com/"+id {
				t.Fatalf("revision %q: %q %t", revision, link, ok)
			}
		}
	}
}

func TestOpenTargetRejectsMalformedIDs(t *testing.T) {
	for _, id := range []string{"", "https://evil.test", "Org/repo/pull/0", "Org/repo/pull/-1", "Org/repo/pull/1?x=y", "Org/repo/pull/1#fragment", "Org/repo/pull/1/more", "Org/repo/settings/1", "../repo/pull/1", "Org/repo/pull/", "Org/repo/pull/1%2f", "Org/repo/pull/1\n"} {
		if link, ok := openTarget(dashboard.Event{ID: id}); ok {
			t.Fatalf("accepted %q as %s", id, link)
		}
	}
}
