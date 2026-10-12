package run

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/siemenvdn/zigbeat/internal/cli"
	"github.com/siemenvdn/zigbeat/internal/config"
)

// offsetStep is one + or -.
const offsetStep = config.OffsetStepMS * time.Millisecond

// saveWord, typed during the show, writes the offset into the config.
const saveWord = "save"

var offsetHelp = fmt.Sprintf("type + or - and Enter to move the lights %d ms later or earlier, +N or -N to move them N ms, a number in ms to set the offset, or %s to keep it in the config", offsetStep.Milliseconds(), saveWord)

// nextOffset is the audio offset after a line typed during the show: each +
// or - moves it offsetStep, a signed number moves it that many ms, a number
// sets it, an empty line keeps it.
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
		if line[0] == '+' || line[0] == '-' {
			next += time.Duration(ms) * time.Millisecond
		} else {
			next = time.Duration(ms) * time.Millisecond
		}
	}
	if ms := next.Milliseconds(); ms < config.MinOffsetMS || ms > config.MaxOffsetMS {
		return cur, fmt.Errorf("audio offset must be %d to %d ms", config.MinOffsetMS, config.MaxOffsetMS)
	}
	return next, nil
}

// calibration follows the audio offset typed during the show and the one last
// saved to the config. Its reader runs in a goroutine of its own.
type calibration struct {
	path       string
	mu         sync.Mutex
	cur, saved time.Duration
}

func newCalibration(path string, offset time.Duration) *calibration {
	return &calibration{path: path, cur: offset, saved: offset}
}

// read sends every new offset typed on r to out until r ends; save writes the
// current one into the config.
func (c *calibration) read(r io.Reader, out chan<- time.Duration) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		if strings.EqualFold(strings.TrimSpace(sc.Text()), saveWord) {
			c.save()
			continue
		}
		c.mu.Lock()
		cur := c.cur
		next, err := nextOffset(sc.Text(), cur)
		if err == nil {
			c.cur = next
		}
		c.mu.Unlock()
		switch {
		case err != nil:
			fmt.Println(err)
		case next == cur:
			fmt.Printf("%s  audio offset %d ms\n", cli.Stamp(), cur.Milliseconds())
		default:
			out <- next
		}
	}
}

// save writes the current offset into budget.audio_offset_ms of the config.
func (c *calibration) save() {
	c.mu.Lock()
	defer c.mu.Unlock()
	ms := c.cur.Milliseconds()
	f, err := config.OpenFile(c.path)
	if err == nil {
		err = f.Set("budget.audio_offset_ms", strconv.FormatInt(ms, 10), "")
	}
	if err == nil {
		err = f.Save(func(data []byte) error {
			_, err := config.Parse(data, os.Getenv)
			return err
		})
	}
	if err != nil {
		fmt.Printf("%s  audio offset %d ms not saved: %v\n", cli.Stamp(), ms, err)
		return
	}
	c.saved = c.cur
	fmt.Printf("%s  audio offset %d ms saved to %s\n", cli.Stamp(), ms, c.path)
}

// unsaved is the hint after the show when the offset changed since the last save.
func (c *calibration) unsaved() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cur == c.saved {
		return ""
	}
	return fmt.Sprintf("audio offset is %d ms, not saved; type %s during the show to keep it", c.cur.Milliseconds(), saveWord)
}
