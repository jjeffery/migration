package migration

import (
	"sort"
)

// A Schema contains all of the information required to perform database
// migrations for a database schema. It contains details about how to
// migrate up to a version from the previous version, and how to migrate
// down from a version to the previous version.
type Schema struct {
	// MigrationsTable specifies the name of the database table used
	// to keep track of the database migrations performed.
	//
	// If not specified, defaults to the constant DefaultMigrationsTable.
	MigrationsTable string

	definitions map[int64]*Definition
	plans       []*migrationPlan
	errs        Errors
}

// Define a database schema version along with the migration up
// from the previous version and the migration down to the
// previous version.
//
// This method is typically called at program initialization, once
// for each database schema version. See the package example.
func (s *Schema) Define(id int64) *Definition {
	d := &Definition{id: id}
	if _, ok := s.definitions[id]; ok {
		s.errs = append(s.errs, &Error{
			Version:     id,
			Description: "defined more than once",
		})
	} else {
		if s.definitions == nil {
			s.definitions = make(map[int64]*Definition)
		}
		s.definitions[id] = d
	}

	// plans are no longer valid after a definition is added
	s.plans = nil

	return d
}

// Err reports a non-nil error if there are any errors in the
// migration schema definition, otherwise it returns nil.
//
// If Err does report a non-nil value, it will be of type Errors.
//
// One common use for this method is to create a simple unit test
// that verifies that checks for errors in the migration schema
// definitions:
//  func TestMigrationSchema(t *testing.T) {
//      if err := schema.Err(); err != nil {
//          t.Fatal(err)
//      }
//  }
func (s *Schema) Err() error {
	s.complete()
	var errs Errors
	errs = append(errs, s.errs...)
	for _, p := range s.plans {
		errs = append(errs, p.errs...)
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

func (s *Schema) complete() {
	if s.plans != nil {
		// already complete
		return
	}

	s.plans = make([]*migrationPlan, 0, len(s.definitions))

	// assemble the version numbers in order
	ids := make([]int64, 0, len(s.definitions))
	{
		for id := range s.definitions {
			ids = append(ids, id)
		}
		sort.Slice(ids, func(i, j int) bool {
			return ids[i] < ids[j]
		})
	}

	var prev *migrationPlan
	for _, id := range ids {
		d := s.definitions[id]
		p := newPlan(d, prev)
		s.plans = append(s.plans, p)
	}
}
