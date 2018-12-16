package migration

import (
	"fmt"
	"strings"

	"github.com/jjeffery/sqlr/private/scanner"
)

// ddlVerb is the verb at the beginning of an SQL DDL statement.
type ddlVerb string

// ddlVerb values
const (
	ddlVerbAlter  = ddlVerb("alter")
	ddlVerbCreate = ddlVerb("create")
	ddlVerbDrop   = ddlVerb("drop")
)

// ddlVerb variables
var (
	// allDDLVerbs is the set of all ddlVerb values.
	allDDLVerbs = map[ddlVerb]struct{}{
		ddlVerbAlter:  struct{}{},
		ddlVerbCreate: struct{}{},
		ddlVerbDrop:   struct{}{},
	}
)

// parseDDLVerb converts a string to a ddlVerb. The boolean
// reports true if s represents a valid ddlVerb.
func parseDDLVerb(s string) (ddlVerb, bool) {
	verb := ddlVerb(s)
	_, ok := allDDLVerbs[verb]
	return verb, ok
}

// dbObjectType represents the supported database object types.
type dbObjectType string

// dbObjectType values
const (
	dbObjectTypeDomain    = dbObjectType("domain")
	dbObjectTypeFunction  = dbObjectType("function")
	dbObjectTypeIndex     = dbObjectType("index")
	dbObjectTypeProcedure = dbObjectType("procedure")
	dbObjectTypeSequence  = dbObjectType("sequence")
	dbObjectTypeTable     = dbObjectType("table")
	dbObjectTypeTrigger   = dbObjectType("trigger")
	dbObjectTypeType      = dbObjectType("type")
	dbObjectTypeView      = dbObjectType("view")
)

// dbObjectType variables
var (
	// allDBObjectTypes is the set of all db object types mapped to
	// a boolean indicating whether it is restorable
	allDBObjectTypes = map[dbObjectType]bool{
		dbObjectTypeDomain:    false,
		dbObjectTypeFunction:  true,
		dbObjectTypeIndex:     false,
		dbObjectTypeProcedure: true,
		dbObjectTypeSequence:  false,
		dbObjectTypeTable:     false,
		dbObjectTypeTrigger:   true,
		dbObjectTypeType:      false,
		dbObjectTypeView:      true,
	}
)

// parseDBObjectType converts a string to a dbObjectType.
// The boolean reports true s represents a valid dbObjectType.
func parseDBObjectType(s string) (dbObjectType, bool) {
	verb := dbObjectType(s)
	_, ok := allDBObjectTypes[verb]
	return verb, ok
}

// ShouldRestore reports whether the automatically generated
// down migration should attempt to restore the previous
// version of the database object.
func (t dbObjectType) ShouldRestore() bool {
	return allDBObjectTypes[t]
}

// ddlActions is a list of ddlAction objects.
type ddlActions []*ddlAction

func newDDLActions(sql string) ddlActions {
	stmts := newStatements(sql)
	var acts ddlActions
	for _, stmt := range stmts {
		act := newDDLAction(stmt)
		if act == nil {
			// found something we don't understand
			return nil
		}
		acts = append(acts, act)
	}

	acts = mergeDropCreate(acts)
	acts = mergeCreateTable(acts)
	return acts
}

func (acts ddlActions) find(verb ddlVerb, objType dbObjectType, schema string, name string) *ddlAction {
	// It seems rather contrived to have a migration that creates a view,
	// drops it and then creates it again, but in this case, we would want to
	// be searching backwards.
	for i := len(acts) - 1; i >= 0; i-- {
		act := acts[i]
		if act.verb == verb &&
			act.objectType == objType &&
			act.schema == schema &&
			act.name == name {
			return act
		}
	}
	return nil
}

// mergeDropCreate looks for a drop followed by a create for the same
// database object. It removes the drop and modifies the create to
// indicate that it has a preceding drop.
func mergeDropCreate(acts ddlActions) ddlActions {
	actions := make(ddlActions, 0, len(acts))
	for i := 0; i < len(acts); {
		action := acts[i]
		if i < len(acts)-1 {
			next := acts[i+1]
			if action.verb == ddlVerbDrop &&
				next.verb == ddlVerbCreate &&
				action.objectType == next.objectType &&
				action.schema == next.schema &&
				action.name == next.name &&
				action.index == next.index {
				// We have a drop and then a create for the same database object.
				// Merge the drop into the create.
				next.dropBefore = true
				next.checkExists = action.checkExists
				actions = append(actions, next)
				i += 2
				continue
			}
		}

		actions = append(actions, action)
		i++
	}

	return actions
}

// mergeCreateTable looks for a create table followed by one or
// more alter table or create index statements for the same table.
// It removes the alter tables and create indexes, because for down migration
// purposes it is as simple as dropping the table.
//
// This function may modify the contents of the src slice on the understanding
// that it will be discarded by the caller.
func mergeCreateTable(src ddlActions) ddlActions {
	var modified bool
	for i := 0; i < len(src); i++ {
		act := src[i]
		if act == nil {
			continue
		}
		if act.verb == ddlVerbCreate && act.objectType == dbObjectTypeTable {
			for j := i + 1; j < len(src); j++ {
				next := src[j]
				if next == nil {
					continue
				}
				if next.verb == ddlVerbAlter &&
					next.objectType == dbObjectTypeTable &&
					next.schema == act.schema &&
					next.name == act.name {
					// found an alter table after the create table
					src[j] = nil
					modified = true
					continue
				}
				if next.verb == ddlVerbCreate &&
					next.objectType == dbObjectTypeIndex &&
					next.schema == act.schema &&
					next.name == act.name {
					// found a create index after the create table
					src[j] = nil
					modified = true
					continue
				}
			}
		}
	}
	if !modified {
		return src
	}

	var dest ddlActions
	for _, act := range src {
		if act != nil {
			dest = append(dest, act)
		}
	}
	return dest
}

// ddlAction provides a summary of the action performed by
// single DDL statetment.
type ddlAction struct {
	verb           ddlVerb
	dropBefore     bool
	checkExists    bool
	checkNotExists bool
	objectType     dbObjectType
	schema         string
	name           string
	index          string // optional index name
}

// qualifiedName reports the name of the database object,
// including the schema if it has been specified.
func (dbo *ddlAction) qualifiedName() string {
	if dbo.schema != "" {
		return fmt.Sprintf("%s.%s", dbo.schema, dbo.name)
	}
	return dbo.name
}

// newDDLAction reports a ddlAction for the statement,
// or nil if the statement contains an unsupported
// command.
func newDDLAction(stmt statement) *ddlAction {
	var obj ddlAction

	if verb, ok := parseDDLVerb(stmt.get(0)); ok {
		obj.verb = verb
		stmt = stmt[1:]
	} else {
		return nil
	}

	if stmt.match("unique", "index") {
		stmt = stmt[1:]
	}

	if objType, ok := parseDBObjectType(stmt.get(0)); ok {
		obj.objectType = objType
		stmt = stmt[1:]
	} else {
		return nil
	}

	stmt = stmt.remove("concurrently")

	if stmt.match("if", "exists") {
		obj.checkExists = true
		stmt = stmt[2:]
	} else if stmt.match("if", "not", "exists") {
		obj.checkNotExists = true
		stmt = stmt[3:]
	}

	if obj.objectType == dbObjectTypeIndex {
		if !stmt.match("on") {
			// index name is specified
			obj.index = stmt.get(0)
			stmt = stmt[1:]
		}
		stmt = stmt.remove("on")
		stmt = stmt.remove("only")
	}

	if stmt.get(1) == "." {
		obj.schema = stmt.get(0)
		obj.name = stmt.get(2)
	} else {
		obj.name = stmt.get(0)
	}

	if obj.name == "" {
		return nil
	}

	return &obj
}

type statement []string

func newStatements(sql string) []statement {
	scan := scanner.New(strings.NewReader(sql))
	scan.IgnoreWhiteSpace = true
	scan.AddKeywords(
		"concurrently",
		"create",
		"domain",
		"drop",
		"exists",
		"function",
		"if",
		"index",
		"on",
		"only",
		"not",
		"procedure",
		"table",
		"trigger",
		"unique",
		"view",
	)

	var stmts []statement
	var stmt statement

	for scan.Scan() {
		lexeme := scan.Text()
		switch scan.Token() {
		case scanner.COMMENT, scanner.WS:
			continue
		case scanner.KEYWORD:
			lexeme = strings.ToLower(lexeme)
		}

		stmt = append(stmt, lexeme)
		if lexeme == ";" {
			stmts = append(stmts, stmt)
			stmt = nil
		}
	}

	if len(stmt) > 0 {
		stmts = append(stmts, stmt)
		stmt = nil
	}

	return stmts
}

func (stmt statement) get(i int) string {
	if len(stmt) <= i {
		return ""
	}
	return stmt[i]
}

func (stmt statement) match(lexemes ...string) bool {
	if len(stmt) < len(lexemes) {
		return false
	}
	for i, lexeme := range lexemes {
		if stmt[i] != lexeme {
			return false
		}
	}
	return true
}

func (stmt statement) remove(lexemes ...string) statement {
	if stmt.match(lexemes...) {
		return stmt[len(lexemes):]
	}
	return stmt
}
