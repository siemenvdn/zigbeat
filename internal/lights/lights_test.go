package lights

import (
	"context"
	"strings"
	"testing"

	"github.com/siemenvdn/zigbeat/internal/z2m"
)

var names = []string{"lamp-1", "lamp-2", "lamp-3"}

func TestSnapshotLeavesOutSilentLights(t *testing.T) {
	c := &fakeConn{misses: map[string]int{"lamp-1": 1, "lamp-2": 2}}
	snaps := Snapshot(context.Background(), c, names, 1000)
	if _, ok := snaps["lamp-1"]; !ok || len(snaps) != 2 {
		t.Errorf("lamp-1 answers the second read, lamp-2 never: %v", snaps)
	}
}

func TestRestoreRetries(t *testing.T) {
	c := &fakeConn{misses: map[string]int{"lamp-2": 2}}
	snaps := map[string]z2m.State{"lamp-1": {State: "ON"}, "lamp-2": {State: "ON"}, "lamp-3": {State: "ON"}}
	if left := Restore(c, snaps, names, 1000); left != nil {
		t.Fatalf("left %v", left)
	}
	if got := strings.Join(c.sets, ","); got != "lamp-1,lamp-2,lamp-2,lamp-2,lamp-3" {
		t.Errorf("sets %s", got)
	}
}

func TestRestoreReturnsWhatIsLeft(t *testing.T) {
	c := &fakeConn{offlineAt: 2}
	snaps := map[string]z2m.State{"lamp-1": {State: "ON"}, "lamp-2": {State: "ON"}, "lamp-3": {State: "ON"}}
	if left := Restore(c, snaps, names, 1000); strings.Join(left, ",") != "lamp-2,lamp-3" {
		t.Errorf("the broker went away before lamp-2: left %v", left)
	}
}
