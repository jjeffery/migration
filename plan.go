package migration

import (
	"fmt"
)

// a migrationPlan contains the information required to
// migrate to a version from the previous version, and back
// down again.
type migrationPlan struct {
	id   VersionID
	up   action
	down action
	errs Errors
}

func newPlan(def *Definition, plans map[VersionID]*migrationPlan) *migrationPlan {
	p := &migrationPlan{
		id:   def.id,
		errs: def.errs(),
	}

	if def.upAction != nil {
		def.upAction(&p.up)
	}
	if def.downAction != nil {
		def.downAction(&p.down)
	}

	addError := func(s string) {
		p.errs = append(p.errs, &Error{
			Version:     p.id,
			Description: s,
		})
	}

	replayUp := func(a *action) {
		if a.replayUp != nil {
			replayID := *a.replayUp
			if replayID >= p.id {
				addError("replay must specify an earlier version")
				return
			}
			prevPlan := plans[replayID]
			if prevPlan == nil {
				addError(fmt.Sprintf("replay refers to unknown version %d", replayID))
			} else {
				*a = prevPlan.up
			}
		}
	}

	replayUp(&p.up)
	replayUp(&p.down)

	return p
}
