package setup

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
)

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

func (in *input) ask(ctx context.Context, w io.Writer, q string) (string, bool) {
	fmt.Fprint(w, q+" ")
	select {
	case l, ok := <-in.lines:
		return strings.TrimSpace(l), ok
	case <-ctx.Done():
		return "", false
	}
}
