package sema

import (
	"github.com/vertex-language/vcx/types"
)

// CheckAccess reports whether a member of record, declared with the given
// access, may be named from currentScope.
func CheckAccess(record *types.Record, access types.Access, currentScope *Scope) bool {
	if access == types.AccessPublic || record == nil {
		return true
	}

	if isFriendOf(record, currentScope) {
		return true
	}

	curRecord := currentScope.InnermostRecord()
	if curRecord == nil {
		// Not a friend, and not inside any class.
		return false
	}

	// A member of the class reaches its own private and protected members.
	if curRecord == record {
		return true
	}

	// A nested class has access to enclosing class members.
	for cur := currentScope; cur != nil; cur = cur.Parent {
		if cur.Kind == ClassScope && cur.Entity == record {
			return true
		}
	}

	// Protected access is granted to derived classes.
	if access == types.AccessProtected && types.IsBaseOf(record, curRecord) {
		return true
	}

	return false
}

// isFriendOf reports whether currentScope was granted friendship by record.
func isFriendOf(record *types.Record, currentScope *Scope) bool {
	if record == nil || currentScope == nil {
		return false
	}

	if fn := currentScope.InnermostFunc(); fn != nil {
		name := fn.Name()
		for _, m := range record.Methods {
			if m.Friend && m.Name == name {
				return true
			}
		}
		for _, friend := range record.FriendFuncs {
			if friend == name {
				return true
			}
		}
	}

	// A friend class reaches the members from any of its own members, so the
	// question is which class the naming context is in, not which function.
	if cur := currentScope.InnermostRecord(); cur != nil && cur != record {
		for _, name := range record.FriendClasses {
			if name == cur.Name {
				return true
			}
		}
	}

	return false
}
