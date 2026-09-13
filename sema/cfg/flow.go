package cfg

import (
	"fmt"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/types"
)

// CheckReturns verifies that all execution paths in a non-void function return a value.
func CheckReturns(cfg *CFG, funcName string, retType types.Type) []string {
	if cfg == nil || types.IsVoid(retType) {
		return nil
	}

	reachable := computeReachable(cfg)

	// Check each predecessor of the Exit block
	var diags []string
	for _, edge := range cfg.Exit.Preds {
		if !reachable[edge.From] {
			continue
		}
		if edge.Kind == EdgeFallthrough {
			diags = append(diags, fmt.Sprintf("non-void function %q does not return a value on all control paths", funcName))
			break
		}
	}

	return diags
}

// CheckUnreachable finds statements in basic blocks that cannot be reached from Entry.
func CheckUnreachable(cfg *CFG) []ast.Tok {
	if cfg == nil {
		return nil
	}

	reachable := computeReachable(cfg)
	var unreachablePos []ast.Tok

	for _, blk := range cfg.Blocks {
		if blk == cfg.Exit || reachable[blk] {
			continue
		}
		if len(blk.Stmts) > 0 {
			unreachablePos = append(unreachablePos, blk.Stmts[0].Pos())
		} else if blk.Terminator != nil {
			unreachablePos = append(unreachablePos, blk.TerminatorPos)
		}
	}

	return unreachablePos
}

// DestructionOrder returns the local objects that must be destroyed along each exiting edge.
func DestructionOrder(cfg *CFG) map[*Edge][]any {
	orders := make(map[*Edge][]any)
	if cfg == nil {
		return orders
	}

	for _, blk := range cfg.Blocks {
		for _, succ := range blk.Succs {
			if succ.Kind == EdgeReturn || succ.Kind == EdgeJump {
				// Reverse order of block locals
				var rev []any
				for i := len(blk.Locals) - 1; i >= 0; i-- {
					rev = append(rev, blk.Locals[i])
				}
				orders[succ] = rev
			}
		}
	}

	return orders
}

func computeReachable(cfg *CFG) map[*BasicBlock]bool {
	reachable := make(map[*BasicBlock]bool)
	if cfg == nil || cfg.Entry == nil {
		return reachable
	}

	queue := []*BasicBlock{cfg.Entry}
	reachable[cfg.Entry] = true

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		for _, edge := range cur.Succs {
			if !reachable[edge.To] {
				reachable[edge.To] = true
				queue = append(queue, edge.To)
			}
		}
	}

	return reachable
}
