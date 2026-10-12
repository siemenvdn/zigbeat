package setup

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
)

func TestAsk(t *testing.T) {
	var out bytes.Buffer
	ctx := context.Background()
	in := newInput(strings.NewReader("1-3\n  \n2\n"))
	for _, want := range []string{"1-3", "", "2"} {
		if got, ok := in.ask(ctx, &out, "q?"); !ok || got != want {
			t.Errorf("got %q, %v; want %q", got, ok, want)
		}
	}
	if _, ok := in.ask(ctx, &out, "q?"); ok {
		t.Error("the end of the input is not an answer")
	}
	r, w := io.Pipe()
	defer w.Close()
	stopped, cancel := context.WithCancel(ctx)
	cancel()
	if _, ok := newInput(r).ask(stopped, &out, "q?"); ok {
		t.Error("Ctrl-C must end the question")
	}
}
