package cli

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRedial(t *testing.T) {
	defer func(d time.Duration) { redialEvery = d }(redialEvery)
	redialEvery = time.Millisecond
	tries := 0
	ok := Redial(context.Background(), "test reconnect", func() error {
		if tries++; tries < 3 {
			return errors.New("refused")
		}
		return nil
	})
	if !ok || tries != 3 {
		t.Errorf("ok %v after %d tries", ok, tries)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if Redial(ctx, "test reconnect", func() error { t.Error("dialled after ctx was done"); return nil }) {
		t.Error("a done ctx must end the redial")
	}
}
