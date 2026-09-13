package cfg

import (
	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/token"
)

// EdgeKind classifies the control transfer between two basic blocks.
type EdgeKind uint8

const (
	EdgeFallthrough EdgeKind = iota
	EdgeBranchTrue
	EdgeBranchFalse
	EdgeReturn
	EdgeThrow
	EdgeJump
)

func (k EdgeKind) String() string {
	switch k {
	case EdgeFallthrough:
		return "fallthrough"
	case EdgeBranchTrue:
		return "true"
	case EdgeBranchFalse:
		return "false"
	case EdgeReturn:
		return "return"
	case EdgeThrow:
		return "throw"
	case EdgeJump:
		return "jump"
	}
	return "unknown"
}

// Edge represents a directed control-flow transfer between basic blocks.
type Edge struct {
	From *BasicBlock
	To   *BasicBlock
	Kind EdgeKind
}

// BasicBlock represents a maximal sequence of linear statements.
type BasicBlock struct {
	ID            int
	Stmts         []ast.Stmt
	Preds         []*Edge
	Succs         []*Edge
	Terminator    ast.Stmt
	TerminatorPos ast.Tok
	Locals        []any
}

// CFG represents the control-flow graph for a function body.
type CFG struct {
	Blocks []*BasicBlock
	Entry  *BasicBlock
	Exit   *BasicBlock
}

// Builder constructs a CFG from AST statements.
type Builder struct {
	cfg      *CFG
	unit     ast.Unit
	curBlock *BasicBlock
	nextID   int
	breakDst *BasicBlock
	contDst  *BasicBlock

	// labels maps label identifiers to their target blocks.
	labels map[string]*BasicBlock
}

// Build constructs a CFG from a function's compound statement body.
func Build(body *ast.CompoundStmt, u ast.Unit) *CFG {
	b := &Builder{
		cfg:    &CFG{},
		unit:   u,
		labels: make(map[string]*BasicBlock),
	}
	b.cfg.Entry = b.newBlock()
	b.cfg.Exit = b.newBlock()
	b.curBlock = b.cfg.Entry

	if body != nil {
		for _, stmt := range body.Stmts {
			b.addStmt(stmt)
		}
	}

	// If current block doesn't terminate, connect it to Exit as fallthrough
	if b.curBlock != nil && len(b.curBlock.Succs) == 0 {
		b.addEdge(b.curBlock, b.cfg.Exit, EdgeFallthrough)
	}

	return b.cfg
}

// labelBlock is the block one identifier label names, made on first mention
// from either the label or a goto to it.
func (b *Builder) labelBlock(name *ast.Ident) *BasicBlock {
	if name == nil || b.unit == nil {
		return nil
	}
	text := name.Text(b.unit)
	if text == "" {
		return nil
	}
	if blk, ok := b.labels[text]; ok {
		return blk
	}
	blk := b.newBlock()
	b.labels[text] = blk
	return blk
}

func (b *Builder) newBlock() *BasicBlock {
	blk := &BasicBlock{
		ID: b.nextID,
	}
	b.nextID++
	b.cfg.Blocks = append(b.cfg.Blocks, blk)
	return blk
}

func (b *Builder) addEdge(from, to *BasicBlock, kind EdgeKind) {
	if from == nil || to == nil {
		return
	}
	edge := &Edge{From: from, To: to, Kind: kind}
	from.Succs = append(from.Succs, edge)
	to.Preds = append(to.Preds, edge)
}

func (b *Builder) addStmt(stmt ast.Stmt) {
	if stmt == nil {
		return
	}
	if b.curBlock == nil {
		b.curBlock = b.newBlock()
	}

	switch s := stmt.(type) {
	case *ast.EmptyStmt:
		return

	case *ast.AttrStmt:
		b.addStmt(s.Stmt)

	case *ast.ExprStmt, *ast.DeclStmt:
		b.curBlock.Stmts = append(b.curBlock.Stmts, s)

	case *ast.CompoundStmt:
		for _, child := range s.Stmts {
			b.addStmt(child)
		}

	case *ast.IfStmt:
		if s.Init != nil {
			b.addStmt(s.Init)
		}
		condBlock := b.curBlock
		thenBlock := b.newBlock()
		elseBlock := b.newBlock()
		joinBlock := b.newBlock()

		b.addEdge(condBlock, thenBlock, EdgeBranchTrue)
		if s.Else != nil {
			b.addEdge(condBlock, elseBlock, EdgeBranchFalse)
		} else {
			b.addEdge(condBlock, joinBlock, EdgeBranchFalse)
		}

		// Then branch
		b.curBlock = thenBlock
		b.addStmt(s.Then)
		if b.curBlock != nil && len(b.curBlock.Succs) == 0 {
			b.addEdge(b.curBlock, joinBlock, EdgeFallthrough)
		}

		// Else branch
		if s.Else != nil {
			b.curBlock = elseBlock
			b.addStmt(s.Else)
			if b.curBlock != nil && len(b.curBlock.Succs) == 0 {
				b.addEdge(b.curBlock, joinBlock, EdgeFallthrough)
			}
		}

		b.curBlock = joinBlock

	case *ast.WhileStmt:
		header := b.newBlock()
		bodyBlock := b.newBlock()
		exitBlock := b.newBlock()

		b.addEdge(b.curBlock, header, EdgeFallthrough)
		b.addEdge(header, bodyBlock, EdgeBranchTrue)
		b.addEdge(header, exitBlock, EdgeBranchFalse)

		oldBreak := b.breakDst
		oldCont := b.contDst
		b.breakDst = exitBlock
		b.contDst = header

		b.curBlock = bodyBlock
		b.addStmt(s.Body)
		if b.curBlock != nil && len(b.curBlock.Succs) == 0 {
			b.addEdge(b.curBlock, header, EdgeJump)
		}

		b.breakDst = oldBreak
		b.contDst = oldCont
		b.curBlock = exitBlock

	case *ast.ForStmt:
		if s.Init != nil {
			b.addStmt(s.Init)
		}
		header := b.newBlock()
		bodyBlock := b.newBlock()
		stepBlock := b.newBlock()
		exitBlock := b.newBlock()

		b.addEdge(b.curBlock, header, EdgeFallthrough)
		b.addEdge(header, bodyBlock, EdgeBranchTrue)
		// An omitted condition is implicitly true, so the header has no false edge.
		if s.Cond != nil {
			b.addEdge(header, exitBlock, EdgeBranchFalse)
		}

		oldBreak := b.breakDst
		oldCont := b.contDst
		b.breakDst = exitBlock
		b.contDst = stepBlock

		b.curBlock = bodyBlock
		b.addStmt(s.Body)
		if b.curBlock != nil && len(b.curBlock.Succs) == 0 {
			b.addEdge(b.curBlock, stepBlock, EdgeFallthrough)
		}

		b.addEdge(stepBlock, header, EdgeJump)

		b.breakDst = oldBreak
		b.contDst = oldCont
		b.curBlock = exitBlock

	case *ast.DoStmt:
		// The body runs before the condition is ever read, so the entry
		// edge goes to the body and `continue` goes to the condition.
		bodyBlock := b.newBlock()
		condBlock := b.newBlock()
		exitBlock := b.newBlock()

		b.addEdge(b.curBlock, bodyBlock, EdgeFallthrough)

		oldBreak := b.breakDst
		oldCont := b.contDst
		b.breakDst = exitBlock
		b.contDst = condBlock

		b.curBlock = bodyBlock
		b.addStmt(s.Body)
		if b.curBlock != nil && len(b.curBlock.Succs) == 0 {
			b.addEdge(b.curBlock, condBlock, EdgeFallthrough)
		}
		b.addEdge(condBlock, bodyBlock, EdgeBranchTrue)
		b.addEdge(condBlock, exitBlock, EdgeBranchFalse)

		b.breakDst = oldBreak
		b.contDst = oldCont
		b.curBlock = exitBlock

	case *ast.SwitchStmt:
		// Switch dispatch: condition branches to case/default labels.
		// Without a default label, the condition may fall through directly to exit.
		if s.Init != nil {
			b.addStmt(s.Init)
		}
		condBlock := b.curBlock
		exitBlock := b.newBlock()

		oldBreak := b.breakDst
		b.breakDst = exitBlock

		var body []ast.Stmt
		switch t := s.Body.(type) {
		case *ast.CompoundStmt:
			body = t.Stmts
		case nil:
		default:
			body = []ast.Stmt{t}
		}

		hasDefault := false
		b.curBlock = nil
		for _, st := range body {
			// A run of labels on one statement names a single block.
			inner := st
			opened := false
			for {
				lbl, ok := inner.(*ast.LabeledStmt)
				if !ok || lbl.Name != nil {
					break
				}
				if !opened {
					caseBlock := b.newBlock()
					if b.curBlock != nil && len(b.curBlock.Succs) == 0 {
						b.addEdge(b.curBlock, caseBlock, EdgeFallthrough)
					}
					b.curBlock = caseBlock
					opened = true
				}
				b.addEdge(condBlock, b.curBlock, EdgeBranchTrue)
				if lbl.Kind == token.DEFAULT {
					hasDefault = true
				}
				inner = lbl.Stmt
			}
			b.addStmt(inner)
		}
		if b.curBlock != nil && len(b.curBlock.Succs) == 0 {
			b.addEdge(b.curBlock, exitBlock, EdgeFallthrough)
		}
		if !hasDefault {
			b.addEdge(condBlock, exitBlock, EdgeBranchFalse)
		}

		b.breakDst = oldBreak
		b.curBlock = exitBlock

	case *ast.LabeledStmt:
		// An identifier label begins a target block for goto statements.
		target := b.labelBlock(s.Name)
		if target != nil {
			if b.curBlock != nil && len(b.curBlock.Succs) == 0 {
				b.addEdge(b.curBlock, target, EdgeFallthrough)
			}
			b.curBlock = target
		}
		b.addStmt(s.Stmt)

	case *ast.GotoStmt:
		// Goto is an unconditional jump that ends the current block.
		if target := b.labelBlock(s.Label); target != nil {
			b.curBlock.Terminator = s
			b.curBlock.TerminatorPos = s.Pos()
			b.addEdge(b.curBlock, target, EdgeJump)
		}
		b.curBlock = nil

	case *ast.RangeForStmt:
		// Range-for header tests begin != end and branches to body or exit.
		if s.Init != nil {
			b.addStmt(s.Init)
		}
		header := b.newBlock()
		bodyBlock := b.newBlock()
		exitBlock := b.newBlock()

		b.addEdge(b.curBlock, header, EdgeFallthrough)
		b.addEdge(header, bodyBlock, EdgeBranchTrue)
		b.addEdge(header, exitBlock, EdgeBranchFalse)

		oldBreak := b.breakDst
		oldCont := b.contDst
		b.breakDst = exitBlock
		b.contDst = header

		b.curBlock = bodyBlock
		b.addStmt(s.Body)
		if b.curBlock != nil && len(b.curBlock.Succs) == 0 {
			b.addEdge(b.curBlock, header, EdgeJump)
		}

		b.breakDst = oldBreak
		b.contDst = oldCont
		b.curBlock = exitBlock

	case *ast.BreakStmt:
		if b.breakDst != nil {
			b.curBlock.Terminator = s
			b.curBlock.TerminatorPos = s.Pos()
			b.addEdge(b.curBlock, b.breakDst, EdgeJump)
		}
		b.curBlock = nil

	case *ast.ContinueStmt:
		if b.contDst != nil {
			b.curBlock.Terminator = s
			b.curBlock.TerminatorPos = s.Pos()
			b.addEdge(b.curBlock, b.contDst, EdgeJump)
		}
		b.curBlock = nil

	case *ast.ReturnStmt:
		b.curBlock.Terminator = s
		b.curBlock.TerminatorPos = s.Pos()
		b.addEdge(b.curBlock, b.cfg.Exit, EdgeReturn)
		b.curBlock = nil
	}
}
