package run

import "testing"

func TestRestoreSurvivesAReconnect(t *testing.T) {
	r, f := newShell(t)
	f.dropAt, f.reconnect = 2, true
	if !r.restore() {
		t.Fatal("the broker came back, so the restore must finish")
	}
	if f.reconnects != 1 || f.sets != 5 {
		t.Errorf("reconnects %d, sets %d: want 1 reconnect and lamp-2 sent again", f.reconnects, f.sets)
	}
}

func TestRestoreReportsAGoneBroker(t *testing.T) {
	r, f := newShell(t)
	f.dropAt = 2
	if r.restore() {
		t.Fatal("the broker stayed away, so the restore must fail")
	}
	if f.reconnects != 1 || f.sets != 2 {
		t.Errorf("reconnects %d, sets %d", f.reconnects, f.sets)
	}
}
