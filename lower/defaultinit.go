package lower

import (
	"github.com/vertex-language/ir"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/sema"
	"github.com/vertex-language/vcx/types"
)

// Default-initialization of a class object: invokes a declared default constructor
// or performs memberwise/basewise default initialization.

// defaultConstruct default-initializes the object of class rec at obj.
func (fl *fn) defaultConstruct(obj ir.Ptr, rec *types.Record, at ast.Tok) bool {
	if fl.blk == nil {
		return false
	}
	// A constructor of its own taking no arguments, or all defaults.
	for _, m := range rec.Methods {
		if m.Name != rec.Name || m.Defaulted || m.Template {
			continue
		}
		if len(m.Func.Params) == 0 || m.Func.Params[0].HasDefault {
			if sym := fl.u.declaredFor(&sema.FuncSymbol{SymName: m.Name, FuncType: m.Func, InClass: rec}); sym != nil {
				fl.constructWith(obj, sym, nil, at)
				return true
			}
		}
	}
	// The implicit one: bases, the table pointer, then the members.
	fieldOffs := make([]int64, len(rec.Fields))
	baseOffs := make([]int64, len(rec.Bases))
	fl.u.model.LayoutWithBases(rec, fieldOffs, baseOffs)
	for i, b := range rec.Bases {
		br := classOf(b.Type)
		if br == nil || b.Virtual {
			continue
		}
		sub := obj
		if baseOffs[i] != 0 {
			sub = fl.blk.Ptr.Add(obj, fl.blk.I64.Const(baseOffs[i]))
		}
		if !fl.defaultConstruct(sub, br, at) {
			return false
		}
	}
	fl.installVPtr(obj, rec)
	return fl.defaultMembers(obj, rec, nil, at)
}

// defaultMembers initializes members of rec at obj not in the skip set
// from default member initializers or via default-initialization for class types.
func (fl *fn) defaultMembers(obj ir.Ptr, rec *types.Record, skip map[string]bool, at ast.Tok) bool {
	fieldOffs := make([]int64, len(rec.Fields))
	fl.u.model.LayoutWithBases(rec, fieldOffs, make([]int64, len(rec.Bases)))
	inits := fl.u.res.Info.MemberInits[rec]
	for i, f := range rec.Fields {
		if f.Name != "" && skip[f.Name] {
			continue
		}
		dst := obj
		if fieldOffs[i] != 0 {
			dst = fl.blk.Ptr.Add(obj, fl.blk.I64.Const(fieldOffs[i]))
		}
		if decl := inits[f.Name]; decl != nil && f.Name != "" {
			if !fl.memberDefault(dst, f.Type, decl) {
				return false
			}
			continue
		}
		if fl.blk == nil {
			return false
		}
		if arr, isArr := types.Unqualify(f.Type).(*types.Array); isArr {
			if er := classOf(elementOf(f.Type)); er != nil && fl.u.needsConstruction(er) {
				fl.defaultElements(dst, arr, at)
			}
			continue
		}
		if fr := classOf(f.Type); fr != nil && fl.u.needsConstruction(fr) {
			if !fl.defaultConstruct(dst, fr, at) {
				return false
			}
		}
	}
	return true
}

// defaultElements default-initializes each element of an array member.
func (fl *fn) defaultElements(dst ir.Ptr, arr *types.Array, at ast.Tok) {
	elemSize, _ := fl.u.sizeAlign(arr.Elem)
	for i := int64(0); i < arr.Len; i++ {
		at2 := dst
		if i > 0 {
			at2 = fl.blk.Ptr.Add(dst, fl.blk.I64.Const(i*elemSize))
		}
		if inner, isArr := types.Unqualify(arr.Elem).(*types.Array); isArr {
			fl.defaultElements(at2, inner, at)
			continue
		}
		if er := classOf(arr.Elem); er != nil {
			fl.defaultConstruct(at2, er, at)
		}
	}
}

// memberDefault initializes a member from its default member initializer (`= expr` or `{ ... }`).
func (fl *fn) memberDefault(dst ir.Ptr, t types.Type, decl *ast.InitDeclarator) bool {
	if ctor := fl.u.res.Info.Ctors[decl]; ctor != nil {
		fl.construct(dst, classOf(t), ctor, decl)
		return true
	}
	if isReference(t) {
		if decl.Value == nil {
			return true
		}
		if addr, ok := fl.bind(decl.Value, t); ok {
			fl.blk.Ptr.Store(addr, dst)
		}
		return true
	}
	if decl.Braced != nil {
		fl.initList(dst, t, decl.Braced)
		return true
	}
	if decl.Value == nil {
		return true
	}
	if rec := classOf(t); rec != nil {
		return fl.exprInto(dst, decl.Value, rec)
	}
	if arr, isArr := types.Unqualify(t).(*types.Array); isArr {
		if lit, isLit := unparen(decl.Value).(*ast.StringLit); isLit {
			return fl.initArrayFromString(dst, arr, lit)
		}
	}
	v, converted := fl.convertedScalar(decl.Value)
	if !converted {
		v = fl.expr(decl.Value)
	}
	if v == nil {
		return false
	}
	fl.store(dst, fl.convert(v, fl.typeOf(decl.Value), t), t)
	return true
}

// memberDefaultsAfter initializes members omitted from a constructor's mem-initializer-list.
func (fl *fn) memberDefaultsAfter(sym *sema.FuncSymbol) {
	named := map[string]bool{}
	for _, mi := range sym.Decl.Inits {
		if mi.Name != nil {
			named[sema.NameString(mi.Name, fl.u.unit)] = true
		}
	}
	fl.defaultMembers(fl.this, sym.InClass, named, sym.SymPos)
}
