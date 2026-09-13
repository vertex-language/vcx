package lower

import (
	"strings"

	"github.com/vertex-language/ir"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/sema"
	"github.com/vertex-language/vcx/types"
)

// Dynamic initialization of namespace-scope objects.
// Emits an initialization function running dynamic initializers in declaration
// order, registered with the platform's startup mechanism (.init_array, .CRT$XCU, or __mod_init_func).

// dynamicInit is one global the initializer function has to construct.
type dynamicInit struct {
	sym  *sema.VarSymbol
	g    *ir.Global
	decl *ast.InitDeclarator
}

// noteDynamicInit records a global whose initializer needs code.
func (u *unit) noteDynamicInit(v *sema.VarSymbol, g *ir.Global) {
	decl := u.declOf(v)
	if decl == nil {
		return
	}
	needsCode := false
	rec := classOf(v.SymType)
	_, isArr := types.Unqualify(v.SymType).(*types.Array)
	switch {
	case u.res.Info.Ctors[decl] != nil:
		needsCode = true
	case decl.Braced != nil && (rec != nil || isArr):
		// An aggregate whose initializer is all constants is emitted in the data section;
		// non-constant initializers are run before main.
		if image, ok := u.constantInit(v.SymType, decl.Braced); ok {
			g.Init(image)
		} else {
			needsCode = true
		}
	case isArr && decl.Value != nil:
		if lit, isLit := unparen(decl.Value).(*ast.StringLit); isLit {
			if image, ok := u.constantInit(v.SymType, lit); ok {
				g.Init(image)
				break
			}
		}
		needsCode = true
	case rec != nil:
		// A class with no constructor: default-initialized before main
		// when that does anything (a table pointer, a member initializer),
		// or copied from its initializer.
		needsCode = u.needsConstruction(rec) || decl.Value != nil
	case v.Init != nil:
		_, err := u.evalInt(v.Init)
		needsCode = err != nil
	}
	if needsCode {
		u.dynamicInits = append(u.dynamicInits, dynamicInit{sym: v, g: g, decl: decl})
	}
}

// declOf is the declarator a namespace-scope variable was defined by.
func (u *unit) declOf(v *sema.VarSymbol) *ast.InitDeclarator {
	if u.declsOf == nil {
		u.declsOf = map[sema.Symbol]*ast.InitDeclarator{}
		for decl, sym := range u.res.Info.Defs {
			if id, isInit := decl.(*ast.InitDeclarator); isInit {
				u.declsOf[sym] = id
			}
		}
	}
	return u.declsOf[v]
}

// defineInitializer emits the unit's initializer function and registers
// it with the target's startup, if any global needs one.
func (u *unit) defineInitializer() {
	if len(u.dynamicInits) == 0 || u.mod.Err() != nil {
		return
	}

	// Initializers run in declaration order within the translation unit.
	inits := u.dynamicInits
	for i := 1; i < len(inits); i++ {
		for j := i; j > 0 && inits[j].decl.Pos() < inits[j-1].decl.Pos(); j-- {
			inits[j], inits[j-1] = inits[j-1], inits[j]
		}
	}

	name := "__vcx_init_" + identOf(u.opt.Name)
	f := u.mod.Func(u.symbolName(name)).Internal()
	stub := &sema.FuncSymbol{SymName: name, FuncType: &types.Func{Ret: types.Typ(types.Void)}}
	fl := &fn{u: u, sym: stub, f: f, slots: map[sema.Symbol]ir.Ptr{}}
	fl.entry = f.Entry()
	fl.blk = f.Block("body")

	for _, di := range inits {
		obj := fl.blk.Ptr.GetAddr(di.g)
		if ctor := u.res.Info.Ctors[di.decl]; ctor != nil {
			fl.construct(obj, classOf(di.sym.SymType), ctor, di.decl)
			continue
		}
		if di.decl.Braced != nil {
			fl.initList(obj, di.sym.SymType, di.decl.Braced)
			fl.endFullExpr()
			continue
		}
		if rec := classOf(di.sym.SymType); rec != nil {
			if di.decl.Value != nil {
				fl.exprInto(obj, di.decl.Value, rec)
			} else {
				fl.defaultConstruct(obj, rec, di.decl.Pos())
			}
			fl.endFullExpr()
			continue
		}
		if arr, isArr := types.Unqualify(di.sym.SymType).(*types.Array); isArr && di.decl.Value != nil {
			if lit, isLit := unparen(di.decl.Value).(*ast.StringLit); isLit {
				fl.initArrayFromString(obj, arr, lit)
				continue
			}
		}
		if v := fl.expr(di.sym.Init); v != nil {
			fl.store(obj, fl.convert(v, fl.typeOf(di.sym.Init), di.sym.SymType), di.sym.SymType)
		}
	}
	if fl.blk != nil {
		fl.blk.Return()
	}
	fl.entry.Br(f.Blocks()[1].To())

	// The pointer the startup code finds. Read-only where the platform
	// allows -- the CRT reads .CRT$XCU and never writes it -- and plain
	// data where the section's convention is writable.
	sec, kind := u.initSection()
	reg := u.mod.Global(u.symbolName("__vcx_init_ptr_"+identOf(u.opt.Name)), kind, ir.StorePtr.FType()).Internal()
	reg.Align(uint64(u.model.SizePtr))
	reg.Init(ir.RelocInit(f))
	reg.Section(sec)
}

// initSection is where the target's startup looks for initializers.
func (u *unit) initSection() (string, ir.Domain) {
	use := u.opt.Target.Use()
	switch {
	case strings.HasSuffix(use, "/windows"):
		return ".CRT$XCU", ir.RO
	case strings.HasSuffix(use, "/macos"):
		return "__DATA,__mod_init_func,mod_init_funcs", ir.RW
	}
	return ".init_array", ir.RW
}
