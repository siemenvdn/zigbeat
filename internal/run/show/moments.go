package show

// interval is the beats between moments: the pace. A beat is a moment when its
// index is a multiple of it, so moments stay on the grid when the pace changes,
// and a pickup beat of a new track is a moment only on that track's grid.
func (p *Planner) interval() int { return p.pace }
