package migration

import "fmt"

type dbObjectType string

const (
	dbObjectTypeView      = dbObjectType("view")
	dbObjectTypeTrigger   = dbObjectType("trigger")
	dbObjectTypeProcedure = dbObjectType("procedure")
	dbObjectTypeTable     = dbObjectType("table")
	dbObjectTypeDomain    = dbObjectType("domain")
)

func (t dbObjectType) restoreLast() bool {
	switch t {
	case dbObjectTypeView, dbObjectTypeTrigger, dbObjectTypeProcedure:
		return true
	}
	return false
}

type dbObject struct {
	objectType dbObjectType
	schema     string
	name       string
}

func parseSQL(s string) *dbObject {
	return nil
}

// String implements the Stringer interface. It is used
// to generate version descriptions.
func (dbo *dbObject) String() string {
	return fmt.Sprintf("%s %s", dbo.objectType, dbo.qualifiedName())
}

func (dbo *dbObject) qualifiedName() string {
	if dbo.schema != "" {
		return fmt.Sprintf("%s.%s", dbo.schema, dbo.name)
	}
	return dbo.name
}

func (dbo *dbObject) dropSQL() string {
	return fmt.Sprintf("drop %s %s;\n", dbo.objectType, dbo.qualifiedName())
}

func (dbo *dbObject) equal(other *dbObject) bool {
	if dbo == nil || other == nil {
		// nil != nil in this case
		return false
	}
	return *dbo == *other
}
