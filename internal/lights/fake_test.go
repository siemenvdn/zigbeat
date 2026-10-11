package lights

import (
	"context"
	"time"

	"github.com/siemenvdn/zigbeat/internal/z2m"
)

// fakeConn answers at once, except for the first misses of a light, and goes
// offline at the offlineAt-th call to Online.
type fakeConn struct {
	misses    map[string]int
	offlineAt int
	online    int
	sets      []string
}

func (f *fakeConn) Get(string, any) error { return nil }

func (f *fakeConn) Set(light string, _ any) error {
	f.sets = append(f.sets, light)
	return nil
}

func (f *fakeConn) Await(_ context.Context, light string, _ time.Time, _ time.Duration) (z2m.State, bool) {
	if f.misses[light] > 0 {
		f.misses[light]--
		return z2m.State{}, false
	}
	return z2m.State{State: "ON"}, true
}

func (f *fakeConn) Online() bool {
	f.online++
	return f.offlineAt == 0 || f.online < f.offlineAt
}
