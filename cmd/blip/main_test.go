package main

import (
	"github.com/BrianLeishman/blip/dashboard"
	"testing"
)

func TestOpenTargetIndependentOfSnapshot(t *testing.T) {
	for _, revision := range []string{"", "old-before-restart", "current"} {
		for _, id := range []string{"ExampleOrg/widget/pull/57", "BrianLeishman/blip/issues/1", "Org/widget/pull/7#issuecomment-12", "Org/widget/pull/7#discussion_r13", "Org/widget/pull/7#pullrequestreview-14", "Org/widget/issues/8#issuecomment-15", "Org/widget/actions/runs/16"} {
			link, ok := openTarget(dashboard.Event{Revision: revision, ID: id})
			if !ok || link != "https://github.com/"+id {
				t.Fatalf("revision %q: %q %t", revision, link, ok)
			}
		}
	}
}

func TestOpenTargetRejectsMalformedIDs(t *testing.T) {
	for _, id := range []string{"", "https://evil.test", "Org/repo/pull/0", "Org/repo/pull/-1", "Org/repo/pull/1?x=y", "Org/repo/pull/1#fragment", "Org/repo/pull/1/more", "Org/repo/settings/1", "../repo/pull/1", "Org/repo/pull/", "Org/repo/pull/1%2f", "Org/repo/pull/1\n", "Org/repo/pull/1#issuecomment-0", "Org/repo/pull/1#issuecomment-2?evil", "Org/repo/issues/1#discussion_r2", "Org/repo/actions/runs/0", "Org/repo/actions/runs/2/extra", "Org/repo/actions/jobs/2", "Org/repo/actions/runs/2#evil"} {
		if link, ok := openTarget(dashboard.Event{ID: id}); ok {
			t.Fatalf("accepted %q as %s", id, link)
		}
	}
}
