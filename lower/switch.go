package lower

import (
	"github.com/vertex-language/ir"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/token"
)

// switchStmt lowers a switch statement.
//
// The condition branches to the matching case label and statements fall
// through in source order until a break or exit. Dispatch is emitted as a chain
// of comparisons.
func (fl *fn) switchStmt(s *ast.SwitchStmt) {
	if s.Init != nil {
		fl.stmt(s.Init)
		if fl.blk == nil {
			return
		}
	}

	cond, ok := s.Cond.(ast.Expr)
	if !ok {
		fl.u.errorf(s.Pos(), "lowering does not handle a declaration as a switch condition yet")
		return
	}
	sel := fl.expr(cond)
	if sel == nil {
		return
	}
	fl.endFullExpr()
	selI32, isI32 := sel.(ir.I32)
	if !isI32 {
		fl.u.errorf(s.Pos(), "lowering handles a switch only on a 32-bit integer so far")
		return
	}

	groups := fl.caseGroups(s)
	if len(groups) == 0 {
		// A switch with no labels runs nothing; the condition is evaluated.
		return
	}

	exit := fl.block("switch_exit")

	// One block per group, made before the dispatch so a test can name one
	// and so each body knows the one that follows it.
	bodies := make([]*ir.Block, len(groups))
	for i := range groups {
		bodies[i] = fl.block("switch_case")
	}

	dflt := exit
	for i, g := range groups {
		if g.isDefault {
			dflt = bodies[i]
		}
	}

	// The dispatch chain. Each test compares against one label and falls to
	// the next test; the last falls to the default, or out.
	for i, g := range groups {
		for _, v := range g.values {
			next := fl.block("switch_test")
			hit := fl.blk.I32.Eq(selI32, fl.blk.I32.Const(v))
			fl.blk.BrIf(hit, bodies[i].To(), next.To())
			fl.blk = next
		}
	}
	fl.blk.Br(dflt.To())

	// The bodies, in source order, each falling into the next.
	oldBreak := fl.breakTo
	fl.breakTo = exit
	for i, g := range groups {
		fl.blk = bodies[i]
		for _, st := range g.stmts {
			fl.stmt(st)
		}
		if fl.blk == nil {
			continue
		}
		if i+1 < len(bodies) {
			fl.blk.Br(bodies[i+1].To())
			continue
		}
		fl.blk.Br(exit.To())
	}
	fl.breakTo = oldBreak

	fl.blk = exit
}

// caseGroup is one run of labels and the statements that follow it.
type caseGroup struct {
	values    []int64
	isDefault bool
	stmts     []ast.Stmt
}

// caseGroups splits a switch body into the groups the dispatch jumps to.
//
// A run of labels on one statement -- `case 1: case 2: x;` -- is a single
// group with two values, because both enter at the same place.
func (fl *fn) caseGroups(s *ast.SwitchStmt) []caseGroup {
	var body []ast.Stmt
	switch t := s.Body.(type) {
	case *ast.CompoundStmt:
		body = t.Stmts
	case nil:
	default:
		body = []ast.Stmt{t}
	}

	var groups []caseGroup
	for _, st := range body {
		inner := st
		opened := false

		for {
			lbl, isLabel := inner.(*ast.LabeledStmt)
			if !isLabel || lbl.Name != nil {
				break
			}
			if !opened {
				groups = append(groups, caseGroup{})
				opened = true
			}
			g := &groups[len(groups)-1]
			if lbl.Kind == token.DEFAULT {
				g.isDefault = true
			} else if lbl.Value != nil {
				v, err := fl.u.evalInt(lbl.Value)
				if err != nil {
					fl.u.errorf(lbl.Pos(), "a case label must be a constant expression: %v", err)
				} else {
					g.values = append(g.values, v)
				}
			}
			inner = lbl.Stmt
		}

		if len(groups) == 0 {
			// Statements before the first label are unreachable and skipped.
			continue
		}
		g := &groups[len(groups)-1]
		g.stmts = append(g.stmts, inner)
	}
	return groups
}
