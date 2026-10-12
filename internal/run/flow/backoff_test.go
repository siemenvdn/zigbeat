package flow

import (
	"testing"
	"time"
)

func TestBackoffSteps(t *testing.T) {
	var b Backoff
	if !b.Signal(t0, PauseLevel) || b.level != 1 {
		t.Fatal("first signal raises to level 1")
	}
	if b.Signal(at(5_000), PauseLevel) || b.level != 1 {
		t.Fatal("a second signal within 10 s must not raise again")
	}
	if !b.Signal(at(10_000), PauseLevel) || b.level != 2 {
		t.Fatal("10 s later a signal raises to level 2")
	}
	if b.Tick(at(65_000)) {
		t.Fatal("recovery needs 60 s since the last signal at 10 s")
	}
	if !b.Tick(at(70_000)) || b.level != 1 {
		t.Fatal("60 s quiet lowers to level 1")
	}
	if b.Tick(at(100_000)) {
		t.Fatal("the next recovery needs another 60 s")
	}
	if !b.Tick(at(130_000)) || b.level != 0 {
		t.Fatal("another 60 s lowers to level 0")
	}
}

func TestBackoffPausesAndResumes(t *testing.T) {
	var b Backoff
	for i := 0; i < 5; i++ {
		b.Signal(t0.Add(time.Duration(i)*stepEvery), PauseLevel)
	}
	if b.level != PauseLevel {
		t.Fatalf("level %d, want the pause level %d", b.level, PauseLevel)
	}
	if !b.Tick(at(100_000)) || b.level != PauseLevel-1 {
		t.Fatalf("a quiet minute resumes the show one level below the pause, got %d", b.level)
	}
	b.Pause(at(110_000))
	if b.level != PauseLevel || b.Signal(at(115_000), PauseLevel) {
		t.Fatal("pause jumps straight to the pause level and stays there")
	}
}

func TestBackoffOtherErrorsCapAtOne(t *testing.T) {
	var b Backoff
	for i := 0; i < 5; i++ {
		b.Signal(t0.Add(time.Duration(i)*stepEvery), 1)
	}
	if b.level != 1 {
		t.Fatalf("errors about other devices may only reach level 1, got %d", b.level)
	}
}

func TestOtherErrorsDoNotHoldAPause(t *testing.T) {
	var b Backoff
	b.Pause(t0)
	for ms := 30_000; ms <= 300_000; ms += 30_000 {
		b.Signal(at(ms), 1)
		b.Tick(at(ms))
	}
	if b.level >= PauseLevel {
		t.Fatalf("errors about other devices kept the pause for 5 minutes: level %d", b.level)
	}
}
