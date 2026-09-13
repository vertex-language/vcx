package sema

import (
	"fmt"
	"strings"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/types"
)

// outOfLineMember recognizes out-of-line member function definitions (e.g. `void Widget::set(...) { ... }`).
// It returns the enclosing record symbol and the member's unqualified name.
func (a *Analyzer) outOfLineMember(d *ast.FuncDecl) (*RecordSymbol, string) {
	if d.Decl == nil {
		return nil, ""
	}
	if qn, isQualified := d.Decl.DeclName().(*ast.QualifiedName); isQualified {
		rs := a.qualifierRecord(qn)
		if rs == nil {
			return nil, ""
		}
		return rs, NameString(qn.Name, a.unit)
	}
	if d.Decl.DeclName() != nil {
		return nil, ""
	}
	for _, spec := range d.Specs.List {
		nt, isNamed := spec.(*ast.NamedTypeSpec)
		if !isNamed {
			continue
		}
		qn, isQualified := nt.Name.(*ast.QualifiedName)
		if !isQualified || len(qn.Qual) == 0 {
			return nil, ""
		}
		// `Widget::Widget` is the constructor and `Widget::~Widget` the
		// destructor; anything else is a qualified type with an unnamed
		// declarator, which is not a definition of anything.
		last := NameString(qn.Name, a.unit)
		class := NameString(qn.Qual[len(qn.Qual)-1], a.unit)
		if last != class && last != "~"+class {
			return nil, ""
		}
		rs := a.qualifierRecord(qn)
		if rs == nil {
			return nil, ""
		}
		return rs, last
	}
	return nil, ""
}

// qualifierRecord is the class a qualified name's qualifier denotes.
func (a *Analyzer) qualifierRecord(qn *ast.QualifiedName) *RecordSymbol {
	if len(qn.Qual) == 0 {
		return nil
	}
	class := &ast.QualifiedName{
		Span:   qn.Span,
		Global: qn.Global,
		Qual:   qn.Qual[:len(qn.Qual)-1],
		Name:   qn.Qual[len(qn.Qual)-1],
	}
	var syms []Symbol
	if len(class.Qual) == 0 && !class.Global.IsValid() {
		syms = LookupUnqualified(a.curScope, NameString(class.Name, a.unit))
	} else {
		syms = ResolveQualifiedName(class, a.curScope, a.globalScope, a.unit)
	}
	for _, s := range syms {
		if rs, isRec := s.(*RecordSymbol); isRec && rs.ClassScope != nil {
			return rs
		}
	}
	return nil
}

// checkOutOfLineMember attaches a definition to the member it defines and
// checks the body in the class's scope, where `x` means `this->x` and the
// parameter types are read as the declaration read them.
func (a *Analyzer) checkOutOfLineMember(d *ast.FuncDecl, rs *RecordSymbol, name string) {
	rec := rs.Record

	// The declared type is built in the class's scope, since a parameter
	// may name a nested type; the constructor and destructor get the
	// return type the class gives them, none.
	oldScope := a.curScope
	a.curScope = rs.ClassScope
	declInfo := BuildDeclSpecs(d.Specs, a.curScope, a.unit)
	declared, _ := BuildDeclarator(d.Decl, declInfo.Type, a.curScope, a.unit).(*types.Func)
	a.curScope = oldScope
	if declared == nil {
		a.errorAt(d.Pos(), "function definition with non-function type")
		return
	}
	if name == rec.Name || name == "~"+rec.Name {
		declared.Ret = types.Typ(types.Void)
	}
	if target := a.conversionTarget(d.Decl.DeclName(), rs.ClassScope); target != nil {
		declared.Ret = target
	}

	var fnSym *FuncSymbol
	for _, s := range rs.ClassScope.LookupLocal(name) {
		fn, isFn := s.(*FuncSymbol)
		if isFn && fn.InClass == rec && sameDeclaration(name, fn.FuncType, declared) {
			fnSym = fn
			break
		}
	}
	if fnSym == nil {
		a.errorAt(d.Pos(), fmt.Sprintf("%s::%s does not match any declaration in the class", rec.Name, name))
		return
	}
	if fnSym.Body != nil {
		a.errorAt(d.Pos(), fmt.Sprintf("redefinition of %s::%s", rec.Name, name))
		return
	}

	// The definition supplies what the declaration could not: the
	// parameter names, the body, the mem-initializers.
	fnSym.FuncType.Params = declared.Params
	fnSym.Decl = d
	if defs := extractDefaults(d.Decl); len(defs) > 0 {
		if len(fnSym.Defaults) < len(defs) {
			newDef := make([]ast.Expr, len(defs))
			copy(newDef, fnSym.Defaults)
			fnSym.Defaults = newDef
		}
		for i, def := range defs {
			if i < len(fnSym.Defaults) && fnSym.Defaults[i] == nil {
				fnSym.Defaults[i] = def
			}
		}
	}
	if comp, isComp := d.Body.(*ast.CompoundStmt); isComp {
		fnSym.Body = comp
	}
	if a.curTemplateParams == nil {
		a.functions = append(a.functions, fnSym)
	}

	fnScope := NewScope(rs.ClassScope, FunctionScope, fnSym)
	oldFunc, oldRecord := a.curFunc, a.curRecord
	a.curScope, a.curFunc, a.curRecord = fnScope, fnSym, rec

	fnSym.Params = nil
	for _, p := range declared.Params {
		pVar := &VarSymbol{SymName: p.Name, SymType: p.Type, SymScope: fnScope, IsParam: true}
		if p.Name != "" {
			fnScope.Insert(pVar)
		}
		fnSym.Params = append(fnSym.Params, pVar)
	}
	a.checkMemInits(d)
	a.CheckStmt(d.Body)

	a.curScope, a.curFunc, a.curRecord = oldScope, oldFunc, oldRecord
}

// sameDeclaration reports whether a definition's type matches a declaration's,
// including the target return type for conversion functions.
func sameDeclaration(name string, declared, defined *types.Func) bool {
	if !types.SameSignature(declared, defined) {
		return false
	}
	if strings.HasPrefix(name, "operator ") {
		return declared.Ret != nil && defined.Ret != nil && declared.Ret.Equal(defined.Ret)
	}
	return true
}

// conversionTarget extracts the target type from a conversion-function-id (operator T).
func (a *Analyzer) conversionTarget(name ast.Name, scope *Scope) types.Type {
	if qn, isQualified := name.(*ast.QualifiedName); isQualified {
		name = qn.Name
	}
	cn, isConv := name.(*ast.ConversionName)
	if !isConv || cn.Type == nil {
		return nil
	}
	info := BuildDeclSpecs(cn.Type.Specs, scope, a.unit)
	return BuildDeclarator(cn.Type.Decl, info.Type, scope, a.unit)
}
