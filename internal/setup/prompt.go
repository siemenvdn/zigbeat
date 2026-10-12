package setup

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
)

// input reads stdin in the background, so a question also ends on Ctrl-C.
// One scanner serves every question: a second one would lose what the first
// had already buffered.
type input struct {
	lines chan string
}

func newInput(r io.Reader) *input {
	in := &input{lines: make(chan string)}
	go func() {
		sc := bufio.NewScanner(r)
		for sc.Scan() {
			in.lines <- sc.Text()
		}
		close(in.lines)
	}()
	return in
}

// ask prints q and returns the answer; false at the end of the input or when
// ctx is done. An empty answer is a valid one, the end of the input is not.
func (in *input) ask(ctx context.Context, w io.Writer, q string) (string, bool) {
	fmt.Fprint(w, q+" ")
	select {
	case l, ok := <-in.lines:
		return strings.TrimSpace(l), ok
	case <-ctx.Done():
		return "", false
	}
}
