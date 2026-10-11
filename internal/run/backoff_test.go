package run

import (
	"testing"
	"time"
)

func TestBackoffSteps(t *testing.T) {
	var b backoff
	if !b.signal(t0, pauseLevel) || b.level != 1 {
		t.Fatal("first signal raises to level 1")
	}
	if b.signal(at(5_000), pauseLevel) || b.level != 1 {
		t.Fatal("a second signal within 10 s must not raise again")
	}
	if !b.signal(at(10_000), pauseLevel) || b.level != 2 {
		t.Fatal("10 s later a signal raises to level 2")
	}
	if b.tick(at(65_000)) {
		t.Fatal("recovery needs 60 s since the last signal at 10 s")
	}
	if !b.tick(at(70_000)) || b.level != 1 {
		t.Fatal("60 s quiet lowers to level 1")
	}
	if b.tick(at(100_000)) {
		t.Fatal("the next recovery needs another 60 s")
	}
	if !b.tick(at(130_000)) || b.level != 0 {
		t.Fatal("another 60 s lowers to level 0")
	}
}

func TestBackoffPausesAndResumes(t *testing.T) {
	var b backoff
	for i := 0; i < 5; i++ {
		b.signal(t0.Add(time.Duration(i)*stepEvery), pauseLevel)
	}
	if b.level != pauseLevel {
		t.Fatalf("level %d, want the pause level %d", b.level, pauseLevel)
	}
	if !b.tick(at(100_000)) || b.level != 2 {
		t.Fatalf("a quiet minute resumes the show at level 2, got %d", b.level)
	}
	b.pause(at(110_000))
	if b.level != pauseLevel || b.signal(at(115_000), pauseLevel) {
		t.Fatal("pause jumps straight to the pause level and stays there")
	}
}

func TestBackoffOtherErrorsCapAtOne(t *testing.T) {
	var b backoff
	for i := 0; i < 5; i++ {
		b.signal(t0.Add(time.Duration(i)*stepEvery), 1)
	}
	if b.level != 1 {
		t.Fatalf("errors about other devices may only reach level 1, got %d", b.level)
	}
}

func TestOtherErrorsDoNotHoldAPause(t *testing.T) {
	var b backoff
	b.pause(t0)
	for ms := 30_000; ms <= 300_000; ms += 30_000 {
		b.signal(at(ms), 1)
		b.tick(at(ms))
	}
	if b.level >= pauseLevel {
		t.Fatalf("errors about other devices kept the pause for 5 minutes: level %d", b.level)
	}
}

func TestNamedErrors(t *testing.T) {
	var b backoff
	if b.namedError(t0) || b.namedError(at(20_000)) {
		t.Fatal("two errors must not pause")
	}
	if b.namedError(at(70_000)) {
		t.Fatal("the first error is older than a minute, so this is only two")
	}
	if !b.namedError(at(75_000)) {
		t.Fatal("three errors within a minute pause the show")
	}
}
