package migration

import (
	"fmt"
	"strings"
)

// a migrationPlan contains the information required to
// migrate to a version from the previous version, and back
// down again.
type migrationPlan struct {
	id   int64
	def  *Definition
	prev *migrationPlan
	errs Errors

	downSQL string
	actions ddlActions
}

func newPlan(def *Definition, prev *migrationPlan) *migrationPlan {
	p := &migrationPlan{
		id:   def.id,
		def:  def,
		prev: prev,
		errs: def.errs(),
	}

	p.actions = newDDLActions(def.upSQL)
	p.deriveDownSQL()
	p.checkErrors()

	return p
}

func (p *migrationPlan) deriveDownSQL() {
	if len(p.def.downMethods()) > 0 {
		// down migration already defined
		return
	}
	if len(p.actions) == 0 {
		// cannot derive down SQL
		return
	}
	if p.actions.containsVerb(ddlVerbAlter, ddlVerbDrop) {
		// cannot reverse a drop or an alter
		return
	}

	// count the number actions that should be restored
	var (
		shouldRestore int
		shouldDrop    int
	)
	for _, act := range p.actions {
		if act.objectType.ShouldRestore() {
			shouldRestore++
		} else {
			shouldDrop++
		}
	}

	if shouldRestore > 1 || (shouldRestore > 0 && shouldDrop > 0) {
		// restorable db objects (views, triggers, etc) need to be
		// in their own version so that they can be automatically restored
		for _, act := range p.actions {
			if act.objectType.ShouldRestore() {
				p.errs = append(p.errs, &Error{
					Version:     p.def.id,
					Description: fmt.Sprintf("create %s %s in its own migration", act.objectType, act.qualifiedName()),
				})
			}
		}
		return
	}

	if shouldRestore > 0 {
		p.deriveDownSQLRestore()
	} else {
		p.deriveDownSQLDrop()
	}
}

func (p *migrationPlan) deriveDownSQLRestore() {
	act := p.actions[0]

	dropSQL := func() string {
		return fmt.Sprintf("drop %s %s;\n\n", act.objectType, act.qualifiedName())
	}

	var stmts []string

	for prev := p.prev; prev != nil; prev = prev.prev {
		prevAct := prev.actions.find(ddlVerbCreate, act.objectType, act.schema, act.name)
		if prevAct != nil {
			// found a previous definition, so use it
			if !prevAct.dropBefore {
				// The previous migration does not drop before-hand, so we have to add it.
				stmts = append(stmts, dropSQL())
			}
			stmts = append(stmts, prev.def.upSQL)
			break
		}
	}

	if len(stmts) == 0 {
		stmts = append(stmts, dropSQL())
	}
	p.downSQL = strings.Join(stmts, "")
}

func (p *migrationPlan) deriveDownSQLDrop() {
	var stmts []string

	// build drop statements in reverse order

	for i := len(p.actions) - 1; i >= 0; i-- {
		act := p.actions[i]
		stmt := fmt.Sprintf("drop %s %s;\n", act.objectType, act.qualifiedName())
		stmts = append(stmts, stmt)
	}

	p.downSQL = strings.Join(stmts, "")
}

func (p *migrationPlan) checkErrors() {
	addError := func(s string) {
		p.errs = append(p.errs, &Error{
			Version:     p.def.id,
			Description: s,
		})
	}

	{
		downMethods := p.def.downMethods()
		if len(downMethods) == 0 && p.downSQL == "" {
			addError("call one method of [Down DownDB DownTx]")
		}
	}
}
