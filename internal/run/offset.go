package run

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/siemenvdn/zigbeat/internal/cli"
	"github.com/siemenvdn/zigbeat/internal/config"
)

// offsetStep is one + or -: about what an ear can tell apart.
const offsetStep = 10 * time.Millisecond

var offsetHelp = fmt.Sprintf("type + or - and Enter to move the lights %d ms later or earlier, or a number in ms", offsetStep.Milliseconds())

// nextOffset is the audio offset after a line typed during the show: each +
// or - moves it offsetStep, a number sets it, an empty line keeps it.
func nextOffset(line string, cur time.Duration) (time.Duration, error) {
	line = strings.TrimSpace(line)
	next := cur
	switch {
	case line == "":
	case strings.Trim(line, "+-") == "":
		next += time.Duration(strings.Count(line, "+")-strings.Count(line, "-")) * offsetStep
	default:
		ms, err := strconv.Atoi(line)
		if err != nil {
			return cur, errors.New(offsetHelp)
		}
		next = time.Duration(ms) * time.Millisecond
	}
	if ms := next.Milliseconds(); ms < config.MinOffsetMS || ms > config.MaxOffsetMS {
		return cur, fmt.Errorf("audio offset must be %d to %d ms", config.MinOffsetMS, config.MaxOffsetMS)
	}
	return next, nil
}

// readOffsets sends every new offset typed on r to out, starting from cur, until r ends.
func readOffsets(r io.Reader, cur time.Duration, out chan<- time.Duration) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		next, err := nextOffset(sc.Text(), cur)
		switch {
		case err != nil:
			fmt.Println(err)
		case next == cur:
			fmt.Printf("%s  audio offset %d ms\n", cli.Stamp(), cur.Milliseconds())
		default:
			cur = next
			out <- next
		}
	}
}
