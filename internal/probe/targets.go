package probe

import (
	"fmt"
	"os"

	"github.com/siemenvdn/zigbeat/internal/z2m"
)

// refuseTargets stops the probe before anything changes when a target may not
// be driven by the rules of zigbeat setup. Without bridge/devices it warns and goes on.
func (s *session) refuseTargets() error {
	devs, ok := s.bridge.Devices(s.ctx)
	if !ok {
		if s.ctx.Err() == nil {
			fmt.Fprintf(os.Stderr, "warning: no %s/bridge/devices; the targets were not checked against it\n", s.cfg.base)
		}
		return nil
	}
	return checkTargets(devs, s.cfg.targets, s.cfg.mode)
}

// checkTargets checks every target in the role its mode needs: brightness
// mode drives any light, color and combo need xy colour.
func checkTargets(devs []z2m.Device, targets []string, mode string) error {
	role := "color"
	if mode == "brightness" {
		role = "breathe"
	}
	for _, t := range targets {
		if err := z2m.Check(devs, t, role); err != nil {
			return err
		}
	}
	return nil
}
