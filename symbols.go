package vcx

import (
	"fmt"
	"sort"

	"github.com/vertex-language/vcx/mangle"
	"github.com/vertex-language/vcx/parser"
	"github.com/vertex-language/vcx/sema"
	"github.com/vertex-language/vcx/token"
	"github.com/vertex-language/vcx/types"
)

// Symbol is a mangled linker symbol with its source spelling.
type Symbol struct {
	// Name is the mangled symbol name.
	Name string

	// Source is the unmangled C++ spelling of the symbol.
	Source string
}

// Symbols runs semantic analysis and returns mangled linker symbol names for all definitions.
func (c *Compiler) Symbols(in Input) ([]Symbol, []Diagnostic, error) {
	file, diags, err := c.Parse(in, parser.DefaultMode)
	if err != nil {
		return nil, nil, err
	}
	defer file.Release()

	tgt, err := c.target()
	if err != nil {
		return nil, diags, err
	}
	model := types.ModelForTarget(tgt.Arch, tgt.OS)

	res, semaDiags := sema.Analyze(file, model)
	for _, d := range semaDiags {
		diags = append(diags, Diagnostic{Severity: d.Severity, Site: d.Site, Message: d.Message})
	}
	if res == nil || HasErrors(diags) {
		return nil, diags, nil
	}

	abi := manglingABI(tgt)
	prefix := symbolPrefix(tgt)
	var out []Symbol

	for _, fn := range res.Functions {
		if fn.Body == nil {
			continue
		}
		name, err := mangle.FunctionName(abi, mangle.Describe(fn))
		if err != nil {
			diags = append(diags, Diagnostic{Severity: token.Error, Site: siteOf(file.Unit, fn.SymPos), Message: err.Error()})
			continue
		}
		out = append(out, Symbol{Name: prefix + name, Source: sourceOfFunc(fn)})
	}

	if res.GlobalScope != nil {
		for _, syms := range res.GlobalScope.Symbols {
			for _, sym := range syms {
				v, isVar := sym.(*sema.VarSymbol)
				if !isVar || v.SymName == "" || !sema.HasExternalLinkage(v) {
					continue
				}
				name, err := mangle.VariableName(abi, mangle.DescribeVariable(v, nil))
				if err != nil {
					diags = append(diags, Diagnostic{Severity: token.Error, Site: siteOf(file.Unit, v.SymPos), Message: err.Error()})
					continue
				}
				out = append(out, Symbol{Name: prefix + name, Source: v.SymName})
			}
		}
		for _, v := range sema.StaticMembers(res) {
			name, err := mangle.VariableName(abi, mangle.DescribeVariable(v, v.InClass))
			if err != nil {
				diags = append(diags, Diagnostic{Severity: token.Error, Site: siteOf(file.Unit, v.SymPos), Message: err.Error()})
				continue
			}
			out = append(out, Symbol{Name: prefix + name, Source: v.InClass.Name + "::" + v.SymName})
		}
		for _, rec := range sema.ConstructedClasses(res) {
			for _, name := range vtableNames(abi, model, rec) {
				out = append(out, Symbol{Name: prefix + name, Source: "vftable for " + rec.Name})
			}
			for _, th := range thunks(model, rec, res) {
				name, err := mangle.ThunkName(abi, mangle.Describe(th.fn), th.adjust)
				if err != nil {
					continue
				}
				out = append(out, Symbol{Name: prefix + name, Source: fmt.Sprintf("adjustor thunk (%d) to %s", th.adjust, sourceOfFunc(th.fn))})
			}
		}
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, diags, nil
}

func sourceOfFunc(fn *sema.FuncSymbol) string {
	s := ""
	if fn.InClass != nil {
		s = fn.InClass.Name + "::"
	} else if fn.SymScope != nil {
		for _, p := range fn.SymScope.Path() {
			s += p + "::"
		}
	}
	s += fn.SymName + "("
	for i, p := range fn.FuncType.Params {
		if i > 0 {
			s += ", "
		}
		s += p.Type.String()
	}
	s += ")"
	if fn.FuncType.Quals&types.QConst != 0 {
		s += " const"
	}
	return s
}

// vtableNames returns mangled symbol names for a class's virtual tables.
func vtableNames(abi mangle.ABI, model types.Model, rec *types.Record) []string {
	tables := model.VTables(rec)
	if abi == mangle.Itanium {
		// Itanium groups all tables under a single symbol.
		if len(tables) == 0 {
			return nil
		}
		name, err := mangle.VTableName(abi, rec, nil)
		if err != nil {
			return nil
		}
		return []string{name}
	}
	var out []string
	for _, vt := range tables {
		var base []*types.Record
		if len(tables) > 1 {
			base = []*types.Record{types.FindBase(rec, vt.Base)}
		}
		name, err := mangle.VTableName(abi, rec, base)
		if err != nil {
			continue
		}
		out = append(out, name)
	}
	return out
}

// thunk describes a this-adjustment thunk for a secondary base table.
type thunk struct {
	fn     *sema.FuncSymbol
	adjust int64
}

// thunks returns all necessary this-adjustment thunks for a class's virtual tables.
func thunks(model types.Model, rec *types.Record, res *sema.Result) []thunk {
	seen := map[thunk]bool{}
	var out []thunk
	for _, table := range model.VTables(rec) {
		for _, slot := range table.Slots {
			adjust := model.ThunkAdjust(rec, table, slot)
			if adjust == 0 {
				continue
			}
			fn := sema.MethodOf(res, types.FindBase(rec, slot.Definer), slot)
			if fn == nil {
				continue
			}
			th := thunk{fn, adjust}
			if !seen[th] {
				seen[th] = true
				out = append(out, th)
			}
		}
	}
	return out
}
