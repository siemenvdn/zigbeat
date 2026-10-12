package show

// spacing returns the beats between moments in force at index, or 0 when
// index is no moment. A new interval starts only on a moment of both the old
// and the new grid, so the breathe fade planned at a moment always ends on the
// next one. A pickup beat (index below 0) of a new track must be on its grid too.
func (p *Planner) spacing(index int) int {
	want := p.pace
	if p.every == 0 {
		p.every = want
	}
	if index%p.every != 0 || index < 0 && index%want != 0 {
		return 0
	}
	if index%want == 0 {
		p.every = want
	}
	return p.every
}
