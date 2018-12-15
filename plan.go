package migration

// a migrationPlan contains the information required to
// migrate to a version from the previous version.
type migrationPlan struct {
	id   int
	def  *Definition
	prev *migrationPlan
	errs Errors

	description string
	downSQL     string
	dbobj       *dbObject
}

func newPlan(def *Definition, prev *migrationPlan) *migrationPlan {
	p := &migrationPlan{
		id:          def.id,
		def:         def,
		prev:        prev,
		errs:        def.errs(),
		description: def.description,
		downSQL:     def.downSQL,
	}

	p.dbobj = parseSQL(def.upSQL)
	p.deriveDescription()
	p.deriveDropSQL()
	p.checkErrors()

	return p
}

func (p *migrationPlan) deriveDescription() {
	if p.description != "" {
		// already have a description
		return
	}
	if p.dbobj == nil {
		// cannot derive a description
		return
	}

	p.description = p.dbobj.String()
}

func (p *migrationPlan) deriveDropSQL() {
	if len(p.def.downMethods()) > 0 {
		// down migration already defined
		return
	}
	if p.dbobj == nil {
		// cannot derive down SQL
		return
	}
	if p.dbobj.objectType.restoreLast() {
		prev := p.prev
		for prev != nil {
			if p.dbobj.equal(prev.dbobj) {
				break
			}
		}
		if prev != nil {
			// TODO(jpj): we need to handle dropping the object
			// prior to recreating it. For now assume that the
			// up migration does this, but we'll need to record
			// this about the migration when we parse it.
			p.downSQL = prev.def.upSQL
			return
		}
	}

	// Either we don't restore this type of object or there is
	// no previous definition to restore. In any case, generate
	// sql to drop the object.
	p.downSQL = p.dbobj.dropSQL()
}

func (p *migrationPlan) checkErrors() {
	addError := func(s string) {
		p.errs = append(p.errs, &Error{
			Version:     p.def.id,
			Description: s,
		})
	}

	if p.description == "" {
		addError("add a description")
	}

	{
		downMethods := p.def.downMethods()
		if len(downMethods) == 0 && p.downSQL == "" {
			addError("call one method of [Down DownDB DownTx]")
		}
	}
}
