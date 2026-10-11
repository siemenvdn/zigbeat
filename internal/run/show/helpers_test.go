package show

import (
	"strings"

	"github.com/siemenvdn/zigbeat/internal/run/runtest"
)

var (
	t0         = runtest.T0
	at         = runtest.At
	testConfig = runtest.Config
	moment     = runtest.Beat
)

// changed lists the colour lights that got a colour in ps.
func changed(ps []Message) string {
	var ls []string
	for _, p := range ps {
		if _, ok := p.Payload["color"]; ok {
			ls = append(ls, p.Light)
		}
	}
	return strings.Join(ls, ",")
}
