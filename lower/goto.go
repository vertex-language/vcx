package lower

// goto and labels ([stmt.goto]). A jump out of a scope destroys what the
// scope holds, as break does; the difficulty is a forward jump, whose
// label -- and so which scopes it leaves -- is not known yet. Such a jump
// goes to a block of its own, with a snapshot of the scopes live at it, and
// when the label is reached that block destroys the scopes between the two
// and branches to it.

import (
	"github.com/vertex-language/ir"

	"github.com/vertex-language/vcx/ast"
)

type label struct {
	blk     *ir.Block
	depth   int // len(fl.scopes) at the label, once it is reached
	reached bool
	pending []pendingGoto
}

type pendingGoto struct {
	blk    *ir.Block
	scopes []*scope // the scopes live at the goto, as they were then
}

func (fl *fn) label(name string) *label {
	if fl.labels == nil {
		fl.labels = map[string]*label{}
	}
	l, ok := fl.labels[name]
	if !ok {
		l = &label{blk: fl.block("label_" + identOf(name))}
		fl.labels[name] = l
	}
	return l
}

func (fl *fn) gotoStmt(s *ast.GotoStmt) {
	if fl.blk == nil || s.Label == nil {
		return
	}
	l := fl.label(s.Label.Text(fl.u.unit))
	if l.reached {
		fl.destroyFrom(l.depth)
		fl.blk.Br(l.blk.To())
		fl.blk = nil
		return
	}
	snap := make([]*scope, len(fl.scopes))
	for i, sc := range fl.scopes {
		snap[i] = &scope{objs: append([]localObj(nil), sc.objs...)}
	}
	t := fl.block("goto")
	fl.blk.Br(t.To())
	l.pending = append(l.pending, pendingGoto{blk: t, scopes: snap})
	fl.blk = nil
}

func (fl *fn) labeledStmt(s *ast.LabeledStmt) {
	l := fl.label(s.Name.Text(fl.u.unit))
	if fl.blk != nil {
		fl.blk.Br(l.blk.To())
	}
	l.reached, l.depth = true, len(fl.scopes)
	for _, p := range l.pending {
		fl.blk = p.blk
		for i := len(p.scopes) - 1; i >= l.depth; i-- {
			fl.destroyScope(p.scopes[i])
		}
		fl.blk.Br(l.blk.To())
	}
	l.pending = nil
	fl.blk = l.blk
	if s.Stmt != nil {
		fl.stmt(s.Stmt)
	}
}

// hasNamedLabel reports whether a goto label is anywhere in s, so that an
// unreachable block that holds one is still lowered.
func hasNamedLabel(s ast.Stmt) bool {
	found := false
	ast.Inspect(s, func(n ast.Node) bool {
		if l, ok := n.(*ast.LabeledStmt); ok && l.Name != nil {
			found = true
		}
		return !found
	})
	return found
}
