package sema

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/token"
	"github.com/vertex-language/vcx/types"
)

// CheckDecl analyzes a declaration node and registers its declared symbols.
func (a *Analyzer) CheckDecl(decl ast.Decl) {
	if decl == nil {
		return
	}

	switch d := decl.(type) {
	case *ast.EmptyDecl:
		return

	case *ast.NamespaceDecl:
		a.checkNamespaceDecl(d)

	case *ast.NamespaceAliasDecl:
		a.checkNamespaceAliasDecl(d)

	case *ast.SimpleDecl:
		a.checkSimpleDecl(d)

	case *ast.FuncDecl:
		a.checkFuncDecl(d)

	case *ast.AliasDecl:
		a.checkAliasDecl(d)

	case *ast.UsingDecl:
		a.checkUsingDecl(d)

	case *ast.UsingDirectiveDecl:
		a.checkUsingDirectiveDecl(d)

	case *ast.StaticAssertDecl:
		a.checkStaticAssertDecl(d)

	case *ast.AccessDecl:
		switch d.Kind {
		case token.PUBLIC:
			a.curAccess = types.AccessPublic
		case token.PROTECTED:
			a.curAccess = types.AccessProtected
		case token.PRIVATE:
			a.curAccess = types.AccessPrivate
		}

	case *ast.LinkageDecl:
		// Language linkage specification.
		old := a.externC
		if d.Lang != nil && len(d.Lang.Segs) > 0 {
			switch a.unit.Text(d.Lang.Segs[0].Lo) {
			case `"C"`:
				a.externC = true
			case `"C++"`:
				a.externC = false
			}
		}
		for _, child := range d.Decls {
			a.CheckDecl(child)
		}
		a.externC = old

	case *ast.StructuredBinding:
		a.checkStructuredBinding(d)

	case *ast.TemplateDecl:
		a.checkTemplateDecl(d)

	case *ast.ExplicitSpecDecl:
		// Explicit specialization of class or variable template.
		if sd, isSimple := d.Decl.(*ast.SimpleDecl); isSimple {
			for _, spec := range sd.Specs.List {
				if cs, isClass := spec.(*ast.ClassSpec); isClass {
					a.checkClassSpec(cs)
				}
			}
			if len(sd.Inits) == 1 && sd.Inits[0].Decl != nil {
				if tn, isTemplate := sd.Inits[0].Decl.DeclName().(*ast.TemplateName); isTemplate {
					a.noteVarSpecialization(sd, tn, nil)
				}
			}
		}

	case *ast.ConceptDecl:
		a.checkConceptDecl(d)

	case *ast.ExportDecl:
		for _, child := range d.Decls {
			a.CheckDecl(child)
		}
	}
}

func (a *Analyzer) checkNamespaceDecl(d *ast.NamespaceDecl) {
	if len(d.Decls) == 1 {
		if alias, isAlias := d.Decls[0].(*ast.NamespaceAliasDecl); isAlias {
			a.checkNamespaceAliasDecl(alias)
			return
		}
	}

	if len(d.Names) == 0 {
		// Anonymous namespace.
		nsScope := NewScope(a.curScope, NamespaceScope, nil)
		a.curScope.AddUsingNamespace(nsScope)
		oldScope := a.curScope
		a.curScope = nsScope
		for _, child := range d.Decls {
			a.CheckDecl(child)
		}
		a.curScope = oldScope
		return
	}

	oldScope := a.curScope
	for i, ident := range d.Names {
		name := ident.Text(a.unit)
		inline := (len(d.Names) == 1 && d.Inline.IsValid()) || (i < len(d.Inlines) && d.Inlines[i].IsValid())
		var nsScope *Scope
		for _, s := range a.curScope.LookupLocal(name) {
			if nsSym, ok := s.(*NamespaceSymbol); ok {
				nsScope = nsSym.InnerScope
				break
			}
		}
		if nsScope == nil {
			nsSym := &NamespaceSymbol{
				SymName:  name,
				SymPos:   ident.Pos(),
				SymScope: a.curScope,
			}
			nsScope = NewScope(a.curScope, NamespaceScope, nsSym)
			nsSym.InnerScope = nsScope
			a.curScope.Insert(nsSym)
		}
		if inline {
			// Reopening an inline namespace does not need inline repeated.
			a.curScope.AddInlineNamespace(nsScope)
		}
		a.curScope = nsScope
	}

	for _, child := range d.Decls {
		a.CheckDecl(child)
	}

	a.curScope = oldScope
}

func (a *Analyzer) checkNamespaceAliasDecl(d *ast.NamespaceAliasDecl) {
	aliasName := d.Name.Text(a.unit)
	var syms []Symbol
	switch t := d.Target.(type) {
	case *ast.Ident:
		syms = LookupUnqualified(a.curScope, t.Text(a.unit))
	case *ast.QualifiedName:
		syms = ResolveQualifiedName(t, a.curScope, a.globalScope, a.unit)
	}
	for _, s := range syms {
		if ns, isNs := s.(*NamespaceSymbol); isNs {
			aliasSym := &NamespaceSymbol{
				SymName:    aliasName,
				SymPos:     d.Pos(),
				SymScope:   a.curScope,
				InnerScope: ns.InnerScope,
			}
			a.curScope.Insert(aliasSym)
			return
		}
	}
	a.errorAt(d.Target.Pos(), fmt.Sprintf("%s is not a namespace", NameString(d.Target, a.unit)))
}

// reportUnresolvedType reports an unknown type name in a declaration.
func (a *Analyzer) reportUnresolvedType(info DeclSpecInfo) {
	if info.Unresolved == "" {
		return
	}
	a.errorAt(info.UnresolvedPos, fmt.Sprintf("unknown type name %q", info.Unresolved))
}

// elaboratedFriendName returns the class name from an elaborated friend declaration.
func elaboratedFriendName(specs *ast.DeclSpecs, u ast.Unit) string {
	if specs == nil {
		return ""
	}
	for _, spec := range specs.List {
		switch t := spec.(type) {
		case *ast.ElaboratedSpec:
			if t.Kind == token.CLASS || t.Kind == token.STRUCT || t.Kind == token.UNION {
				return NameString(t.Name, u)
			}
		case *ast.NamedTypeSpec:
			return NameString(t.Name, u)
		}
	}
	return ""
}

// resolveConstructor resolves the constructor called by a declaration's initializer.
func (a *Analyzer) resolveConstructor(init *ast.InitDeclarator, rec *types.Record) {
	// Aggregate initialization does not call a constructor.
	if init.Braced != nil && !hasUserConstructor(rec) {
		return
	}

	var argExprs []ast.Expr
	switch {
	case len(init.Args) > 0:
		init.Args = a.expandPackArgs(init.Args)
		argExprs = init.Args
	case init.Braced != nil:
		for _, item := range init.Braced.Items {
			if e, isExpr := item.(ast.Expr); isExpr {
				argExprs = append(argExprs, e)
			}
		}
	case init.Value != nil:
		// Direct prvalue initialization needs no constructor call.
		info := a.CheckExpr(init.Value)
		if isDependentExpr(info) {
			return
		}
		if types.AsRecord(types.Unqualify(info.Type)) == rec && info.ValCat == PrValue {
			return
		}
		if types.AsRecord(types.Unqualify(info.Type)) != rec {
			// Converting constructor.
			converting := a.convertingConstructor(rec, Argument{Type: info.Type, IsLValue: info.ValCat == LValue}, init.Pos())
			if converting == nil {
				return
			}
			if !converting.Defaulted {
				a.info.Ctors[init] = converting
			}
			return
		}
		argExprs = []ast.Expr{init.Value}
	}

	args := make([]Argument, 0, len(argExprs))
	for _, e := range argExprs {
		info := a.CheckExpr(e)
		if isDependentExpr(info) && a.dependentContext() {
			// Dependent argument in template context.
			return
		}
		args = append(args, Argument{Type: info.Type, IsLValue: info.ValCat == LValue})
	}

	ctors := a.memberFuncs(rec, rec.Name)
	if len(ctors) == 0 || dependentArguments(args) {
		return
	}

	chosen, err := a.resolveAmong(ctors, nil, args, init.Pos())
	if err != nil {
		if len(argExprs) > 0 {
			a.errorAt(init.Pos(), fmt.Sprintf("no matching constructor for %s: %v", rec.Name, err))
		}
		return
	}

	// Trivial defaulted constructors perform no runtime function calls.
	if chosen.Defaulted {
		return
	}
	if init.Value != nil && len(init.Args) == 0 && len(chosen.Defaults) > 1 {
		init.Args = []ast.Expr{init.Value}
		init.Value = nil
	}
	for i := len(init.Args); i < len(chosen.Defaults); i++ {
		if def := chosen.Defaults[i]; def != nil {
			init.Args = append(init.Args, def)
			a.CheckExpr(def)
		}
	}
	a.info.Ctors[init] = chosen
}

// hasUserConstructor reports whether a class declares any user constructors.
func hasUserConstructor(rec *types.Record) bool {
	for _, m := range rec.Methods {
		if m.Name == rec.Name && !m.Defaulted {
			return true
		}
	}
	for _, base := range rec.InheritedCtors {
		if hasUserConstructor(base) {
			return true
		}
	}
	return false
}

// bitfieldDeclarator finds the `: width` a member declarator carries, past
// whatever the declarator wrapped it in.
func bitfieldDeclarator(d ast.Declarator) (*ast.BitfieldDeclarator, bool) {
	for d != nil {
		switch t := d.(type) {
		case *ast.BitfieldDeclarator:
			return t, true
		case *ast.PointerDeclarator:
			d = t.Inner
		case *ast.ParenDeclarator:
			d = t.Inner
		default:
			return nil, false
		}
	}
	return nil, false
}

// substituteAuto puts a deduced type where the `auto` was, through whatever
// the declarator wrapped it in. `auto v`, `auto& v` and `const auto& v` are
// three different types over the same deduction, and a range-for is written
// with all three.
func substituteAuto(declared, deduced types.Type) types.Type {
	if declared == nil || deduced == nil {
		return declared
	}
	switch t := declared.(type) {
	case *types.Pointer:
		return &types.Pointer{Elem: substituteAuto(t.Elem, deduced)}
	}
	if types.IsLValueReference(declared) {
		inner := types.RemoveReference(declared)
		if types.Unqualify(inner).Kind() != types.AutoKind {
			return declared
		}
		return types.AddLValueReference(types.Qualify(deduced, qualsOf(inner)))
	}
	if types.IsRValueReference(declared) {
		inner := types.RemoveReference(declared)
		if types.Unqualify(inner).Kind() != types.AutoKind {
			return declared
		}
		return types.AddRValueReference(types.Qualify(deduced, qualsOf(inner)))
	}
	if types.Unqualify(declared).Kind() == types.AutoKind {
		return types.Qualify(deduced, qualsOf(declared))
	}
	return declared
}

// qualsOf reads the cv-qualification off a type, so a substitution can put
// it back on what replaces it.
func qualsOf(t types.Type) types.Qual {
	var q types.Qual
	if types.IsConst(t) {
		q |= types.QConst
	}
	if types.IsVolatile(t) {
		q |= types.QVolatile
	}
	return q
}

func (a *Analyzer) checkSimpleDecl(d *ast.SimpleDecl) {
	// Check for class or enum specifiers inside d.Specs
	if d.Specs != nil {
		for _, spec := range d.Specs.List {
			switch s := spec.(type) {
			case *ast.ClassSpec:
				a.checkClassSpec(s)
				// Anonymous union members belong to the enclosing class.
				if s.Name == nil && s.Kind == token.UNION && len(d.Inits) == 0 && a.curRecord != nil && a.curScope.Kind == ClassScope {
					a.adoptAnonymousUnion(s)
				}
			case *ast.EnumSpec:
				a.checkEnumSpec(s)
			case *ast.ElaboratedSpec:
				if len(d.Inits) == 0 && a.curTemplateParams != nil && a.curRecord == nil && !a.instantiating {
					a.declareClassTemplate(s)
				} else if len(d.Inits) == 0 && a.curTemplateParams == nil {
					// Forward declaration of an incomplete class.
					a.declareClass(s)
				} else {
					a.predeclareElaborated(s)
				}
			}
		}
	}

	for _, init := range d.Inits {
		a.predeclareParams(init.Decl)
	}
	declInfo := BuildDeclSpecs(d.Specs, a.curScope, a.unit)

	// Friend class declaration.
	if declInfo.Friend && a.curRecord != nil && len(d.Inits) == 0 {
		if name := elaboratedFriendName(d.Specs, a.unit); name != "" {
			a.curRecord.FriendClasses = append(a.curRecord.FriendClasses, name)
			return
		}
	}

	a.reportUnresolvedType(declInfo)

	for _, init := range d.Inits {
		fullType := BuildDeclarator(init.Decl, declInfo.Type, a.curScope, a.unit)
		name := ""
		if init.Decl != nil && init.Decl.DeclName() != nil {
			name = NameString(init.Decl.DeclName(), a.unit)
		}

		// Variable template partial specialization.
		if tn, isTemplate := init.Decl.DeclName().(*ast.TemplateName); isTemplate && init.Value != nil {
			if !a.instantiating {
				if a.templateOwner == d && a.curTemplateParams != nil {
					a.noteVarSpecialization(d, tn, a.curTemplateParams)
				}
				continue
			}
			name = NameString(tn.Name, a.unit)
		}

		// Typedef declaration.
		if declInfo.Typedef {
			if name == "" {
				continue
			}
			if rec := types.AsRecord(types.Unqualify(fullType)); rec != nil && rec.Name == name {
				isSameRecord := false
				for _, existing := range a.curScope.LookupLocal(name) {
					if rs, isRec := existing.(*RecordSymbol); isRec && rs.Record == rec {
						isSameRecord = true
						break
					}
				}
				if isSameRecord {
					continue
				}
			}
			if err := a.declScope().Insert(&TypeSymbol{
				SymName:  name,
				SymType:  fullType,
				SymPos:   init.Pos(),
				SymScope: a.curScope,
			}); err != nil {
				a.errorAt(init.Pos(), err.Error())
			}
			continue
		}

		if ft, ok := fullType.(*types.Func); ok && declInfo.Friend && a.curRecord != nil && a.curScope.Kind == ClassScope && name != "" {
			// Friend function declared in enclosing namespace.
			a.curRecord.FriendFuncs = append(a.curRecord.FriendFuncs, name)
			into := a.curScope
			for into != nil && into.Kind != NamespaceScope && into.Kind != GlobalScope {
				into = into.Parent
			}
			fnSym := &FuncSymbol{
				Decl: nil, SymName: name, FuncType: ft, SymPos: init.Pos(), SymScope: into,
				Inline: declInfo.Inline || declInfo.Constexpr, Constexpr: declInfo.Constexpr, ExternC: a.externC,
			}
			fnSym.Constraints = a.constraintsOf(d, init.Decl)
			fnSym.ConstraintScope = into
			if surviving, err := into.InsertFunc(fnSym); err == nil {
				fnSym = surviving
			}
			a.noteDeclared(fnSym)
			if a.curTemplateParams != nil && a.templateOwner == d {
				a.noteFunctionTemplate(fnSym, nil)
			}
			continue
		}
		if ft, ok := fullType.(*types.Func); ok {
			// Constructor declarator name fallback to enclosing class name.
			if name == "" && a.curRecord != nil {
				if rec := types.AsRecord(types.Unqualify(declInfo.Type)); rec == a.curRecord {
					name = a.curRecord.Name
				}
			}
			// Constructors and destructors have no return type.
			if a.curRecord != nil && (name == a.curRecord.Name || name == "~"+a.curRecord.Name) {
				ft.Ret = types.Typ(types.Void)
			}
			if target := a.conversionTarget(init.Decl.DeclName(), a.curScope); target != nil {
				ft.Ret = target
			}
			virtual := declInfo.Virtual
			if a.curRecord != nil && !virtual && types.OverridesVirtual(a.curRecord, name, ft) {
				virtual = true
			}

			fnSym := &FuncSymbol{
				SymName:   name,
				FuncType:  ft,
				SymPos:    init.Pos(),
				SymScope:  a.curScope,
				Inline:    declInfo.Inline || declInfo.Constexpr || declInfo.Consteval,
				Constexpr: declInfo.Constexpr,
				Consteval: declInfo.Consteval,
				Virtual:   virtual,
				Static:    declInfo.Storage == StorageStatic && a.curRecord != nil,
				Explicit:  declInfo.Explicit,
				Friend:    declInfo.Friend,
				InClass:   a.curRecord,
				Access:    a.curAccess,
				ExternC:   a.externC,
				Defaults:  extractDefaults(init.Decl),
			}
			a.checkParamDefaults(funcDeclaratorOf(init.Decl), nil)
			fnSym.Constraints = a.constraintsOf(d, init.Decl)
			fnSym.ConstraintScope = a.curScope

			// Member function declared without a body inside a class.
			var method *types.Method
			if a.curRecord != nil {
				// `= 0` on a virtual function marks it pure virtual.
				pure := false
				if virtual && init.Value != nil {
					if lit, isLit := init.Value.(*ast.BasicLit); isLit && lit.Kind == token.INT_LIT && lit.Spelling(a.unit) == "0" {
						pure = true
					}
				}
				method = &types.Method{
					Name:        name,
					Func:        ft,
					Access:      a.curAccess,
					Virtual:     virtual,
					PureVirtual: pure,
					Static:      fnSym.Static,
					Explicit:    declInfo.Explicit,
					Friend:      declInfo.Friend,
					Template:    a.curTemplateParams != nil && a.templateOwner == d,
				}
				a.curRecord.Methods = append(a.curRecord.Methods, method)
				fnSym.Method = method
				fnSym.PureVirtual = pure
			}

			fnSym.AsmLabel = a.asmLabel(init)
			if surviving, err := a.declScope().InsertFunc(fnSym); err != nil {
				a.errorAt(init.Pos(), err.Error())
			} else {
				if surviving.AsmLabel == "" {
					surviving.AsmLabel = fnSym.AsmLabel
				}
				fnSym = surviving
			}
			a.noteDeclared(fnSym)
			if a.curTemplateParams != nil && (a.curRecord == nil || a.templateOwner == d) {
				// Record declared function template parameters and defaults.
				a.noteFunctionTemplate(fnSym, nil)
			}
			if method != nil {
				if a.methodSyms == nil {
					a.methodSyms = map[*types.Method]*FuncSymbol{}
				}
				a.methodSyms[method] = fnSym
			}
			continue
		}

		if init.Braced != nil {
			// List-initialization.
			if rec := types.AsRecord(types.Unqualify(fullType)); rec == nil || !hasUserConstructor(rec) {
				a.checkListInit(init.Braced, fullType)
			}
		}
		if init.Value == nil && len(init.Args) == 1 && types.AsRecord(types.Unqualify(fullType)) == nil {
			init.Value = init.Args[0]
		}
		if init.Value != nil {
			initInfo := a.CheckExpr(init.Value)
			fullType = a.deduceAuto(fullType, initInfo)
			cs := ClassifyConversion(initInfo.Type, fullType, initInfo.ValCat == LValue)
			a.noteConversionFunction(init.Value, initInfo.Type, fullType)
			if lit, isLit := unparenExpr(init.Value).(*ast.StringLit); isLit {
				// String literal initializing a character array.
				if ok, msg := a.stringInitializes(fullType, initInfo.Type); ok {
					cs.Valid = true
				} else if msg != "" {
					a.errorAt(lit.Pos(), msg)
					cs.Valid = true
				}
			}
			if !cs.Valid && !isDependentExpr(initInfo) && !isDependentType(fullType) && !isNullConstant(init.Value, initInfo) {
				if rec := types.AsRecord(types.Unqualify(fullType)); rec == nil || a.convertingConstructor(rec, Argument{Type: initInfo.Type, IsLValue: initInfo.ValCat == LValue}, init.Pos()) == nil {
					a.errorAt(init.Value.Pos(), fmt.Sprintf("cannot convert initializer of type %q to %q", initInfo.Type, fullType))
				}
			}
		} else if a.rangeElem != nil {
			// For-range declaration type deduction.
			fullType = substituteAuto(fullType, a.rangeElem)
		} else if mentionsAuto(fullType) {
			a.errorAt(init.Pos(), "declaration of variable with auto type requires an initializer")
		}

		// Resolve constructor for object of class type.
		if rec := types.AsRecord(types.Unqualify(fullType)); rec != nil && !(a.curRecord != nil && a.curScope.Kind == ClassScope) {
			a.resolveConstructor(init, rec)
		}

		varSym := &VarSymbol{
			SymName:    name,
			SymType:    fullType,
			SymPos:     init.Pos(),
			SymScope:   a.curScope,
			Storage:    declInfo.Storage,
			Constexpr:  declInfo.Constexpr,
			Consteval:  declInfo.Consteval,
			Init:       init.Value,
			BracedInit: init.Braced,
			ExternC:    a.externC,
			Inline:     declInfo.Inline,
			// Defined unless extern without an initializer.
			Defined: declInfo.Storage != StorageExtern || init.Value != nil || init.Braced != nil,
		}
		// Variable template declaration.
		if a.templateOwner == d && a.curTemplateParams != nil && !a.instantiating {
			varSym.Template = &VarTemplate{Params: a.curTemplateParams, Decl: d, Scope: a.curScope}
		}

		// Static data member definition at namespace scope.
		if qn, isQualified := init.Decl.DeclName().(*ast.QualifiedName); isQualified && a.curRecord == nil {
			if member := a.defineStaticMember(qn, init); member != nil {
				a.recordDef(init, member)
			}
			continue
		}

		inClassBody := a.curRecord != nil && a.curScope.Kind == ClassScope
		if inClassBody && declInfo.Storage == StorageStatic {
			// Static data members are defined out-of-line or in-class if inline/constexpr.
			varSym.InClass = a.curRecord
			constInit := types.IsConst(fullType) && (types.IsInteger(types.Unqualify(fullType)) || types.IsEnum(types.Unqualify(fullType))) && (init.Value != nil || init.Braced != nil)
			varSym.Defined = declInfo.Inline || declInfo.Constexpr || constInit
			varSym.Inline = varSym.Defined
		} else if inClassBody {
			// Bit-field width.
			field := types.Field{
				Name:    name,
				Type:    fullType,
				Access:  a.curAccess,
				HasInit: init.Value != nil || init.Braced != nil,
				Align:   a.alignasOf(append(append([]*ast.AttrGroup{}, d.Attrs...), d.Specs.Aligns...)),
			}
			if bf, ok := bitfieldDeclarator(init.Decl); ok {
				field.BitField = true
				if n, err := a.NewConstContext().EvalInt(bf.Width); err == nil && n >= 0 {
					field.Width = n
				}
			}
			a.curRecord.Fields = append(a.curRecord.Fields, field)
			if field.HasInit && a.info != nil && name != "" {
				if a.info.MemberInits[a.curRecord] == nil {
					a.info.MemberInits[a.curRecord] = map[string]*ast.InitDeclarator{}
				}
				a.info.MemberInits[a.curRecord][name] = init
			}
		}

		if name != "" {
			a.recordDef(init, varSym)
			into := a.curScope
			if varSym.Template != nil {
				into = a.declScope()
			}
			if err := into.Insert(varSym); err != nil {
				a.errorAt(init.Pos(), err.Error())
			}
		}
	}
}

// alignasOf returns the requested alignment from alignas specifiers.
func (a *Analyzer) alignasOf(groups []*ast.AttrGroup) int64 {
	var align int64
	for _, g := range groups {
		var n int64
		switch {
		case g.AlignX != nil:
			v, err := a.NewConstContext().EvalInt(g.AlignX)
			if err != nil {
				a.errorAt(g.Pos(), fmt.Sprintf("alignas needs a constant expression: %v", err))
				continue
			}
			n = v
		case g.Align != nil:
			info := BuildDeclSpecs(g.Align.Specs, a.curScope, a.unit)
			t := BuildDeclarator(g.Align.Decl, info.Type, a.curScope, a.unit)
			v, ok := a.model.Alignof(t)
			if !ok {
				a.errorAt(g.Pos(), "alignas of a type whose alignment is not known")
				continue
			}
			n = v
		}
		if n < 0 || n&(n-1) != 0 {
			a.errorAt(g.Pos(), fmt.Sprintf("alignas(%d) is not a power of two", n))
			continue
		}
		if n > align {
			align = n
		}
	}
	return align
}

// stringInitializes checks whether an array of character type can be
// initialized by a string literal, completing unsized arrays or reporting errors.
func (a *Analyzer) stringInitializes(target, lit types.Type) (bool, string) {
	arr, isArr := types.Unqualify(target).(*types.Array)
	from, fromArr := lit.(*types.Array)
	if !isArr || !fromArr {
		return false, ""
	}
	want := types.Unqualify(arr.Elem)
	have := types.Unqualify(from.Elem)
	// An ordinary literal also initializes signed and unsigned char.
	if !want.Equal(have) && !(isPlainChar(want) && isPlainChar(have)) {
		return false, ""
	}
	if arr.Incomplete {
		arr.Incomplete = false
		arr.Len = from.Len
		return true, ""
	}
	if from.Len-1 > arr.Len {
		return false, fmt.Sprintf("a string of %d characters does not fit an array of %d", from.Len-1, arr.Len)
	}
	return true, ""
}

// isPlainChar reports whether t is char, signed char, or unsigned char.
func isPlainChar(t types.Type) bool {
	k, isBasic := t.(*types.Basic)
	return isBasic && (k.Kind() == types.Char || k.Kind() == types.SChar || k.Kind() == types.UChar)
}

// defineStaticMember finds the static data member a qualified declarator
// names and marks it defined.
func (a *Analyzer) defineStaticMember(qn *ast.QualifiedName, init *ast.InitDeclarator) *VarSymbol {
	syms := ResolveQualifiedName(qn, a.curScope, a.globalScope, a.unit)
	for _, s := range syms {
		v, isVar := s.(*VarSymbol)
		if !isVar || v.InClass == nil {
			continue
		}
		if v.Defined {
			a.errorAt(init.Pos(), fmt.Sprintf("redefinition of %s::%s", v.InClass.Name, v.SymName))
			return v
		}
		v.Defined = true
		if init.Value != nil {
			v.Init = init.Value
		}
		return v
	}
	a.errorAt(init.Pos(), fmt.Sprintf("%s does not name a static data member", NameString(qn, a.unit)))
	return nil
}

func (a *Analyzer) checkClassSpec(s *ast.ClassSpec) {
	tag := types.TagStruct
	defaultAccess := types.AccessPublic
	if s.Kind == token.CLASS {
		tag = types.TagClass
		defaultAccess = types.AccessPrivate
	} else if s.Kind == token.UNION {
		tag = types.TagUnion
		defaultAccess = types.AccessPublic
	}

	name := ""
	if s.Name != nil {
		name = NameString(s.Name, a.unit)
	}

	// Class template partial or explicit specialization.
	if _, isSpecialization := s.Name.(*ast.TemplateName); isSpecialization && s.Lbrace.IsValid() && !a.instantiating {
		a.noteSpecialization(s, a.curTemplateParams)
		a.checkSpecializationBody(s)
		return
	}

	rec := &types.Record{
		Tag:    tag,
		Name:   name,
		Scopes: a.curScope.Path(),
		// The alignment ceiling #pragma pack had in effect where the
		// class was written; the layout reads it through MemberAlign.
		Pack:  a.packAt(s.Pos()),
		Final: s.Final.IsValid(),
		// Alignas on the class head.
		Align: a.alignasOf(s.Aligns),
	}

	recSym := &RecordSymbol{
		SymName:  name,
		Record:   rec,
		SymPos:   s.Pos(),
		SymScope: a.curScope,
		Spec:     s,
	}

	// Complete a forward-declared class.
	if a.curTemplateParams == nil && !a.instantiating && name != "" {
		for _, existing := range a.declScope().LookupLocal(name) {
			if rs, isRec := existing.(*RecordSymbol); isRec && rs.ClassTemplate == nil && rs.TemplateOf == nil && !rs.Record.Complete && rs.ClassScope == nil {
				rs.Record.Tag = tag
				rs.Record.Pack = rec.Pack
				rs.Record.Final = rec.Final
				rec, recSym = rs.Record, rs
				break
			}
		}
	}

	// Complete previously declared class template.
	if a.curTemplateParams != nil && a.curRecord == nil && !a.instantiating && name != "" {
		for _, existing := range a.declScope().LookupLocal(name) {
			if rs, isRec := existing.(*RecordSymbol); isRec && rs.ClassTemplate != nil && rs.ClassTemplate.Spec == nil {
				rs.ClassTemplate.Spec = s
				rs.ClassTemplate.Params = a.curTemplateParams
				rs.Record.Tag = tag
				rec, recSym = rs.Record, rs
				break
			}
		}
	}
	if a.instantiating && a.curRecord == nil {
		// Injected-class-name inside specialization.
		recSym.TemplateOf = a.primary
		// The specialization is known by its arguments from here: a
		// member naming `S<T>` -- `using self = S<T>` -- finds the
		// class being made, not a second instantiation of it.
		if a.primary != nil && a.primary.ClassTemplate != nil && a.instArgs != nil {
			rec.TemplateArgs = a.instArgs
			primaryRecords[rec] = a.primary.Record
			a.primary.ClassTemplate.Instances[argsKey(a.instArgs)] = recSym
		}
	}
	// Class template at namespace scope or member template.
	if a.curTemplateParams != nil && (a.curRecord == nil && !a.instantiating || a.ownsClass(s)) && recSym.ClassTemplate == nil {
		recSym.ClassTemplate = &ClassTemplateInfo{
			Params:    a.curTemplateParams,
			Spec:      s,
			Scope:     a.declScope(),
			Instances: map[string]*RecordSymbol{},
		}
	}

	if name != "" && !a.declaredAlready(recSym) {
		a.curScope.Insert(recSym)
		for cur := a.curScope.Parent; cur != nil; cur = cur.Parent {
			if cur.Kind != TemplateParamScope {
				cur.Insert(recSym)
				break
			}
		}
	}

	// Base classes
	for _, b := range s.Bases {
		bAccess := defaultAccess
		if b.AccKind == token.PUBLIC {
			bAccess = types.AccessPublic
		} else if b.AccKind == token.PROTECTED {
			bAccess = types.AccessProtected
		} else if b.AccKind == token.PRIVATE {
			bAccess = types.AccessPrivate
		}

		// Base class type resolution.
		baseName := NameString(b.Name, a.unit)
		specs := &ast.DeclSpecs{Span: b.Span, List: []ast.DeclSpec{&ast.NamedTypeSpec{Span: b.Span, Typename: ast.NoTok, Name: b.Name}}}
		info := BuildDeclSpecs(specs, a.curScope, a.unit)
		baseType := info.Type
		if baseType == nil || info.Unresolved != "" {
			if !a.dependentContext() && !a.instantiating {
				a.errorAt(b.Pos(), fmt.Sprintf("no class named %q", baseName))
			}
			baseType = &types.DependentType{Name: baseName}
		}

		rec.Bases = append(rec.Bases, types.BaseSpec{
			Type:    baseType,
			Access:  bAccess,
			Virtual: b.Virtual.IsValid(),
		})
	}

	classScope := NewScope(a.curScope, ClassScope, rec)
	recSym.ClassScope = classScope
	a.curScope.noteRecord(recSym)
	if name == "" {
		a.lastAnonymousRecord = rec
	}

	// Injected-class-name.
	if name != "" {
		classScope.Insert(recSym)
	}

	oldScope := a.curScope
	oldRecord := a.curRecord
	oldAccess := a.curAccess

	a.curScope = classScope
	a.curRecord = rec
	a.curAccess = defaultAccess

	// Pass 1: declare all member variables and method signatures
	var methodDecls []*ast.FuncDecl
	var friendDecls []*ast.FuncDecl
	var memberTemplates []*ast.TemplateDecl
	for _, member := range s.Members {
		if fn, ok := member.(*ast.FuncDecl); ok && fn.Body != nil && hasFriend(fn.Specs) {
			body := fn.Body
			fn.Body = nil
			a.CheckDecl(fn)
			fn.Body = body
			friendDecls = append(friendDecls, fn)
		} else if fn, ok := member.(*ast.FuncDecl); ok && fn.Body != nil {
			body := fn.Body
			fn.Body = nil
			a.CheckDecl(fn)
			fn.Body = body
			methodDecls = append(methodDecls, fn)
		} else if td, ok := member.(*ast.TemplateDecl); ok && memberTemplateBody(td) != nil {
			// Member template body deferred until complete-class context.
			fn := memberTemplateBody(td)
			body := fn.Body
			fn.Body = nil
			a.CheckDecl(td)
			fn.Body = body
			memberTemplates = append(memberTemplates, td)
		} else {
			a.CheckDecl(member)
		}
	}

	rec.Complete = true
	a.deduceDefaultedComparisons(rec, s.Pos())
	SynthesizeSpecialMembers(rec)

	// Pass 2: analyze deferred method bodies now that all members are declared.
	// In class template specializations, member bodies are only checked when used.
	bodies := func() {
		for _, fn := range methodDecls {
			if a.instantiating && !a.usedByExistence(fn, rec) {
				a.deferBody(fn, rec)
				continue
			}
			a.checkMethodBody(fn)
		}
		for _, fn := range friendDecls {
			// A friend's body, in the namespace it belongs to, now that the
			// class it may reach into is complete.
			a.checkFriendBody(fn)
		}
		for _, td := range memberTemplates {
			// The body is checked under the template-head again, as written:
			// its own parameters are open in it whatever the class's are.
			tScope := NewScope(a.curScope, TemplateParamScope, nil)
			params := a.templateParams(td, tScope)
			prevScope, prevParams, prevOwner := a.curScope, a.curTemplateParams, a.templateOwner
			a.curScope, a.curTemplateParams, a.templateOwner = tScope, params, td.Decl
			a.checkMethodBody(memberTemplateBody(td))
			a.curScope, a.curTemplateParams, a.templateOwner = prevScope, prevParams, prevOwner
		}
	}
	if oldRecord != nil {
		// Nested class member bodies wait for the enclosing class to be complete.
		params, owner, requires, wasInstantiating := a.curTemplateParams, a.templateOwner, a.curTemplateRequires, a.instantiating
		a.nestedBodies = append(a.nestedBodies, func() {
			saved := a.enterInstantiation(classScope)
			savedInstantiating := a.instantiating
			a.instantiating, a.curTemplateParams, a.templateOwner, a.curTemplateRequires = wasInstantiating, params, owner, requires
			a.curRecord, a.curAccess = rec, defaultAccess
			bodies()
			a.instantiating = savedInstantiating
			a.leaveInstantiation(saved)
		})
	} else {
		bodies()
		// The nested classes' bodies, now that this class is complete.
		for len(a.nestedBodies) > 0 {
			pending := a.nestedBodies
			a.nestedBodies = nil
			for _, body := range pending {
				body()
			}
		}
	}

	a.curScope = oldScope
	a.curRecord = oldRecord
	a.curAccess = oldAccess
}

// memberTemplateBody is the function a member template declares with a
// body, or nil when it declares something else.
func memberTemplateBody(td *ast.TemplateDecl) *ast.FuncDecl {
	if fn, isFunc := td.Decl.(*ast.FuncDecl); isFunc && fn.Body != nil {
		return fn
	}
	return nil
}

func (a *Analyzer) checkEnumSpec(s *ast.EnumSpec) {
	name := ""
	if s.Name != nil {
		name = NameString(s.Name, a.unit)
	}

	var underlying types.Type = types.Typ(types.Int)
	if s.Base != nil {
		underlying = BuildDeclSpecs(s.Base, a.curScope, a.unit).Type
	}

	enum := &types.Enum{
		Name:       name,
		Scopes:     a.curScope.Path(),
		Scoped:     s.Scoped.IsValid(),
		Underlying: underlying,
	}

	// Enumerator scope.
	enumScope := NewScope(a.curScope, BlockScope, nil)
	var nextVal int64
	for _, v := range s.Values {
		val := nextVal
		if v.Value != nil {
			// Evaluated as constant expression.
			saved := a.curScope
			a.curScope = enumScope
			info := a.CheckExpr(v.Value)
			n, err := a.NewConstContext().EvalInt(v.Value)
			a.curScope = saved
			if err == nil {
				val = n
			} else if !isDependentExpr(info) && !a.dependentContext() {
				a.errorAt(v.Value.Pos(), fmt.Sprintf("enumerator value is not a constant expression: %v", err))
			}
		}
		item := types.Enumerator{
			Name: v.Name.Text(a.unit),
			Val:  val,
		}
		enum.Enumerators = append(enum.Enumerators, item)
		nextVal = val + 1

		sym := &EnumeratorSymbol{
			SymName:  item.Name,
			Enum:     enum,
			Val:      item.Val,
			SymPos:   v.Pos(),
			SymScope: a.curScope,
		}
		enumScope.Insert(sym)
		if !enum.Scoped {
			// Unscoped enums export enumerators into enclosing scope
			a.curScope.Insert(sym)
		}
	}

	enum.Complete = true
	enumSym := &EnumSymbol{
		SymName:  name,
		Enum:     enum,
		SymPos:   s.Pos(),
		SymScope: a.curScope,
	}

	if name != "" {
		a.curScope.Insert(enumSym)
	}
}

func (a *Analyzer) checkFuncDecl(d *ast.FuncDecl) {
	if a.curRecord == nil {
		if rs, member := a.outOfLineMember(d); rs != nil {
			a.checkOutOfLineMember(d, rs, member)
			return
		}
	}
	// Friend function defined in class belongs to enclosing namespace.
	if a.curRecord != nil && a.curScope.Kind == ClassScope && hasFriend(d.Specs) {
		if d.Decl != nil && d.Decl.DeclName() != nil {
			a.curRecord.FriendFuncs = append(a.curRecord.FriendFuncs, NameString(d.Decl.DeclName(), a.unit))
		}
		into := a.curScope
		for into != nil && into.Kind != NamespaceScope && into.Kind != GlobalScope {
			into = into.Parent
		}
		saved := a.enterInstantiation(into)
		a.instantiating, a.curTemplateParams = false, saved.params
		a.definingFriend = true
		a.checkFuncDecl(d)
		a.definingFriend = false
		a.leaveInstantiation(saved)
		return
	}

	a.predeclareParams(d.Decl)
	declInfo := BuildDeclSpecs(d.Specs, a.curScope, a.unit)
	fullType := BuildDeclarator(d.Decl, declInfo.Type, a.curScope, a.unit)
	ft, ok := fullType.(*types.Func)
	if !ok {
		a.errorAt(d.Pos(), "function definition with non-function type")
		return
	}

	name := ""
	if d.Decl != nil && d.Decl.DeclName() != nil {
		name = NameString(d.Decl.DeclName(), a.unit)
	}

	// Constructor declarator-id matches class name.
	if name == "" && a.curRecord != nil && d.Decl != nil {
		if rec := types.AsRecord(types.Unqualify(declInfo.Type)); rec == a.curRecord {
			name = a.curRecord.Name
		}
	}

	if a.curRecord != nil && (name == a.curRecord.Name || name == "~"+a.curRecord.Name) {
		ft.Ret = types.Typ(types.Void)
	}
	if target := a.conversionTarget(d.Decl.DeclName(), a.curScope); target != nil {
		ft.Ret = target
	}
	if a.curRecord != nil && !declInfo.Virtual && types.OverridesVirtual(a.curRecord, name, ft) {
		// Implicit virtual override.
		declInfo.Virtual = true
	}

	fnSym := &FuncSymbol{
		Decl:     d,
		SymName:  name,
		FuncType: ft,
		SymPos:   d.Pos(),
		SymScope: a.curScope,
		// In-class definitions and constexpr functions are inline.
		Inline:    declInfo.Inline || declInfo.Constexpr || declInfo.Consteval || (a.curRecord != nil && d.Body != nil) || (a.definingFriend && d.Body != nil),
		Constexpr: declInfo.Constexpr,
		Consteval: declInfo.Consteval,
		Virtual:   declInfo.Virtual,
		Static:    declInfo.Storage == StorageStatic && a.curRecord != nil,
		Explicit:  declInfo.Explicit,
		Friend:    declInfo.Friend,
		Defaulted: d.Defaulted.IsValid(),
		Deleted:   d.Deleted.IsValid(),
		InClass:   a.curRecord,
		Access:    a.curAccess,
		ExternC:   a.externC,
		Defaults:  extractDefaults(d.Decl),
	}
	a.checkParamDefaults(funcDeclaratorOf(d.Decl), nil)
	fnSym.Constraints = a.constraintsOf(d, d.Decl)
	fnSym.ConstraintScope = a.curScope

	var method *types.Method
	if a.curRecord != nil {
		method = &types.Method{
			Name:      name,
			Func:      ft,
			Access:    a.curAccess,
			Virtual:   declInfo.Virtual,
			Static:    fnSym.Static,
			Explicit:  declInfo.Explicit,
			Friend:    declInfo.Friend,
			Defaulted: fnSym.Defaulted,
			Deleted:   fnSym.Deleted,
		}
		method.Template = a.curTemplateParams != nil && a.templateOwner == d
		a.curRecord.Methods = append(a.curRecord.Methods, method)
		fnSym.Method = method
	}

	if d.Body != nil {
		if comp, ok := d.Body.(*ast.CompoundStmt); ok {
			fnSym.Body = comp
		}
	}

	if name != "" {
		// Template name declared in template-declaration scope.
		if surviving, err := a.declScope().InsertFunc(fnSym); err == nil {
			fnSym = surviving
		}
		a.noteDeclared(fnSym)
	}
	if method != nil {
		if a.methodSyms == nil {
			a.methodSyms = map[*types.Method]*FuncSymbol{}
		}
		a.methodSyms[method] = fnSym
	}

	// Function template definition recorded for future instantiation.
	if a.curTemplateParams != nil && (a.curRecord == nil || a.templateOwner == d) {
		a.noteFunctionTemplate(fnSym, d)
	}

	if d.Body != nil {
		a.checkFunctionBody(fnSym, d)
	}
}

func (a *Analyzer) checkFunctionBody(fnSym *FuncSymbol, d *ast.FuncDecl) {
	if d == nil || d.Body == nil {
		return
	}
	if comp, ok := d.Body.(*ast.CompoundStmt); ok {
		fnSym.Body = comp
	}
	if fnSym.Template == nil && a.curTemplateParams == nil {
		a.functions = append(a.functions, fnSym)
	}

	fnScope := NewScope(a.curScope, FunctionScope, fnSym)
	oldScope := a.curScope
	oldFunc := a.curFunc

	a.curScope = fnScope
	a.curFunc = fnSym

	// Register parameter variables in function scope
	fnSym.Params = a.declareParams(fnSym.FuncType, fnScope)

	// Constructor initializers
	a.checkMemInits(d)

	a.CheckStmt(d.Body)

	a.curScope = oldScope
	a.curFunc = oldFunc
}

func (a *Analyzer) checkMethodBody(d *ast.FuncDecl) {
	fnSym := a.methodSymbolFor(d)
	if fnSym == nil {
		return
	}

	if comp, ok := d.Body.(*ast.CompoundStmt); ok {
		fnSym.Body = comp
	}
	fnSym.Decl = d
	// Functions defined inside their class are inline.
	fnSym.Inline = true
	// Class template members are lowered with the template's instantiations.
	if a.curTemplateParams == nil {
		a.functions = append(a.functions, fnSym)
	}

	fnScope := NewScope(a.curScope, FunctionScope, fnSym)
	oldScope := a.curScope
	oldFunc := a.curFunc

	a.curScope = fnScope
	a.curFunc = fnSym

	fnSym.Params = a.declareParams(fnSym.FuncType, fnScope)

	a.checkMemInits(d)

	a.CheckStmt(d.Body)

	a.curScope = oldScope
	a.curFunc = oldFunc
}

// methodSymbolFor is the symbol a member function definition inside a
// class belongs to: the overload of its name with its signature, in the
// class scope the analysis is in.
func (a *Analyzer) methodSymbolFor(d *ast.FuncDecl) *FuncSymbol {
	name := ""
	if d.Decl != nil && d.Decl.DeclName() != nil {
		name = NameString(d.Decl.DeclName(), a.unit)
	}
	// Constructors without declarator names take their name from the class.
	if name == "" && a.curRecord != nil && d.Decl != nil {
		info := BuildDeclSpecs(d.Specs, a.curScope, a.unit)
		if rec := types.AsRecord(types.Unqualify(info.Type)); rec == a.curRecord {
			name = a.curRecord.Name
		}
	}

	// Match signature overload.
	declInfo := BuildDeclSpecs(d.Specs, a.curScope, a.unit)
	declared, _ := BuildDeclarator(d.Decl, declInfo.Type, a.curScope, a.unit).(*types.Func)

	// Match declaration by AST node first to handle overloads differing only by constraints.
	for _, s := range a.curScope.LookupLocal(name) {
		if fn, ok := s.(*FuncSymbol); ok && fn.Decl == d {
			return fn
		}
	}
	for _, s := range a.curScope.LookupLocal(name) {
		fn, ok := s.(*FuncSymbol)
		if !ok {
			continue
		}
		if declared != nil && !sameDeclaration(name, fn.FuncType, declared) {
			continue
		}
		return fn
	}
	return nil
}

// declScope is the scope a declaration's own name belongs to,
// walking out past template parameter scopes.
func (a *Analyzer) declScope() *Scope {
	s := a.curScope
	for s != nil && s.Kind == TemplateParamScope {
		s = s.Parent
	}
	if s == nil {
		return a.curScope
	}
	return s
}

func (a *Analyzer) checkAliasDecl(d *ast.AliasDecl) {
	name := d.Name.Text(a.unit)
	targetInfo := BuildDeclSpecs(d.Type.Specs, a.curScope, a.unit)
	targetType := BuildDeclarator(d.Type.Decl, targetInfo.Type, a.curScope, a.unit)

	sym := &TypeSymbol{
		SymName:  name,
		SymType:  targetType,
		SymPos:   d.Pos(),
		SymScope: a.curScope,
	}
	// For alias templates, keep the AST type-id to rebuild for each specialization.
	if a.curTemplateParams != nil && (a.curRecord == nil || a.templateOwner == d) {
		sym.Alias = &AliasTemplate{Params: a.curTemplateParams, Type: d.Type, Scope: a.declScope()}
	}
	if err := a.declScope().Insert(sym); err != nil {
		a.errorAt(d.Pos(), err.Error())
	}
}

func (a *Analyzer) checkUsingDecl(d *ast.UsingDecl) {
	for _, n := range d.Names {
		// Inheriting constructors from base class.
		if qn, isQualified := n.(*ast.QualifiedName); isQualified && a.curRecord != nil && a.curScope.Kind == ClassScope {
			if base := a.inheritedCtorBase(qn); base != nil {
				a.curRecord.InheritedCtors = append(a.curRecord.InheritedCtors, base)
				continue
			}
		}
		// Using-declaration introduces target symbols into current scope.
		var syms []Symbol
		if qn, isQualified := n.(*ast.QualifiedName); isQualified {
			syms = ResolveQualifiedName(qn, a.curScope, a.globalScope, a.unit)
		} else {
			syms = LookupUnqualified(a.curScope, NameString(n, a.unit))
		}
		for _, s := range syms {
			a.curScope.AddUsingDecl(s.Name(), s)
		}
	}
}

// inheritedCtorBase is the direct base a using-declaration of the form
// `Base::Base` names, or nil when the declaration is not that.
func (a *Analyzer) inheritedCtorBase(qn *ast.QualifiedName) *types.Record {
	if len(qn.Qual) == 0 {
		return nil
	}
	last := NameString(qn.Qual[len(qn.Qual)-1], a.unit)
	if NameString(qn.Name, a.unit) != last {
		return nil
	}
	for _, b := range a.curRecord.Bases {
		if br := types.AsRecord(types.Unqualify(b.Type)); br != nil && br.Name == last {
			return br
		}
	}
	return nil
}

func (a *Analyzer) checkUsingDirectiveDecl(d *ast.UsingDirectiveDecl) {
	var syms []Symbol
	switch n := d.Name.(type) {
	case *ast.Ident:
		syms = LookupUnqualified(a.curScope, n.Text(a.unit))
	case *ast.QualifiedName:
		syms = ResolveQualifiedName(n, a.curScope, a.globalScope, a.unit)
	}
	for _, s := range syms {
		if nsSym, ok := s.(*NamespaceSymbol); ok {
			a.curScope.AddUsingNamespace(nsSym.InnerScope)
			return
		}
	}
	a.errorAt(d.Name.Pos(), fmt.Sprintf("%s is not a namespace", NameString(d.Name, a.unit)))
}

func (a *Analyzer) checkStaticAssertDecl(d *ast.StaticAssertDecl) {
	// Defer evaluation in dependent contexts.
	if a.dependentContext() {
		a.CheckExpr(d.Cond)
		return
	}
	// Checked before evaluation for call resolution.
	a.CheckExpr(d.Cond)
	ctx := a.NewConstContext()
	passed, err := ctx.EvalBool(d.Cond)
	if err != nil {
		a.errorAt(d.Pos(), fmt.Sprintf("static assertion expression is not a constant expression: %v", err))
		return
	}
	if !passed {
		msg := "static assertion failed"
		if d.Msg != nil {
			if lit, ok := d.Msg.(*ast.StringLit); ok && len(lit.Segs) > 0 {
				msg = a.unit.Text(lit.Segs[0].Lo)
			}
		}
		a.errorAt(d.Pos(), msg)
	}
}

// ownsClass reports whether the template header being checked is the
// class specifier's own: `template <class U> struct R { ... };` declares
// R through a simple-declaration whose specifiers hold the class.
func (a *Analyzer) ownsClass(s *ast.ClassSpec) bool {
	sd, isSimple := a.templateOwner.(*ast.SimpleDecl)
	if !isSimple || sd.Specs == nil {
		return false
	}
	for _, spec := range sd.Specs.List {
		if spec == s {
			return true
		}
	}
	return false
}

func (a *Analyzer) checkTemplateDecl(d *ast.TemplateDecl) {
	tScope := NewScope(a.curScope, TemplateParamScope, nil)
	oldScope := a.curScope
	a.curScope = tScope
	params := a.templateParams(d, tScope)

	prevParams, prevOwner, prevRequires := a.curTemplateParams, a.templateOwner, a.curTemplateRequires
	a.curTemplateParams, a.templateOwner, a.curTemplateRequires = params, d.Decl, nil
	if d.Requires != nil {
		a.curTemplateRequires = d.Requires.X
	}

	if d.Decl != nil {
		a.CheckDecl(d.Decl)
	}

	a.curTemplateParams, a.templateOwner, a.curTemplateRequires = prevParams, prevOwner, prevRequires
	a.curScope = oldScope
}

// constraintsOf gathers a function declaration's requires-clauses.
func (a *Analyzer) constraintsOf(d ast.Decl, declarator ast.Declarator) []ast.Expr {
	var out []ast.Expr
	if a.curTemplateRequires != nil && a.templateOwner == d {
		out = append(out, a.curTemplateRequires)
	}
	for decl := declarator; decl != nil; {
		switch node := decl.(type) {
		case *ast.FuncDeclarator:
			if node.Requires != nil {
				out = append(out, node.Requires.X)
			}
			decl = nil
		case *ast.PointerDeclarator:
			decl = node.Inner
		case *ast.ParenDeclarator:
			decl = node.Inner
		default:
			decl = nil
		}
	}
	return out
}

// templateParams declares a template-head's parameters in tScope and returns their symbols.
func (a *Analyzer) templateParams(d *ast.TemplateDecl, tScope *Scope) []*TemplateParamSymbol {
	var params []*TemplateParamSymbol
	if d.Params != nil {
		for i, decl := range d.Params.Params {
			pName := ""
			isType := true
			isPack := false
			var pType types.Type
			var dflt ast.Node
			var pDecl *ast.ParamDecl
			switch p := decl.(type) {
			case *ast.TypeParam:
				if p.Name != nil {
					pName = p.Name.Text(a.unit)
				}
				isType = true
				isPack = p.Ellipsis.IsValid()
				if p.Default != nil {
					dflt = p.Default
				}
				// Type template parameter placeholder.
				pType = &types.TemplateParam{
					Name:   pName,
					Index:  i,
					IsType: true,
					IsPack: p.Ellipsis.IsValid(),
				}
			case *ast.TemplateTemplateParam:
				// Template template parameter.
				if p.Name != nil {
					pName = p.Name.Text(a.unit)
				}
				isType = true
				isPack = p.Ellipsis.IsValid()
				pType = &types.TemplateParam{Name: pName, Index: i, IsType: true, IsPack: isPack}
				paramSym := &TemplateParamSymbol{
					SymName: pName, Index: i, IsType: true, IsPack: isPack, IsTemplate: true,
					SymType: pType, SymPos: decl.Pos(), SymScope: tScope,
				}
				if pName != "" {
					tScope.Insert(paramSym)
				}
				params = append(params, paramSym)
				continue
			case *ast.ParamDecl:
				if p.Decl != nil && p.Decl.DeclName() != nil {
					pName = NameString(p.Decl.DeclName(), a.unit)
				}
				isType = false
				// Non-type template parameter declared type.
				pDecl = p
				info := BuildDeclSpecs(p.Specs, tScope, a.unit)
				pType = BuildDeclarator(p.Decl, info.Type, tScope, a.unit)
				if p.Default != nil {
					dflt = p.Default
				}
				isPack = isPackParamDecl(p)
			}
			paramSym := &TemplateParamSymbol{
				SymName:  pName,
				Index:    i,
				IsType:   isType,
				IsPack:   isPack,
				SymType:  pType,
				SymPos:   decl.Pos(),
				SymScope: tScope,
				Default:  dflt,
				Decl:     pDecl,
			}
			if pName != "" {
				tScope.Insert(paramSym)
			}
			params = append(params, paramSym)
		}
	}
	return params
}

func (a *Analyzer) checkConceptDecl(d *ast.ConceptDecl) {
	name := d.Name.Text(a.unit)
	a.declScope().Insert(&ConceptSymbol{
		SymName:    name,
		SymPos:     d.Pos(),
		SymScope:   a.curScope,
		Params:     a.curTemplateParams,
		Constraint: d.Value,
	})
}

// checkMemInits checks a constructor's mem-initializer-list.
func (a *Analyzer) checkMemInits(d *ast.FuncDecl) {
	for _, mi := range d.Inits {
		var args []Argument
		mi.Args = a.expandPackArgs(mi.Args)
		items := mi.Args
		if mi.Braced != nil {
			for _, item := range mi.Braced.Items {
				if x, isExpr := item.(ast.Expr); isExpr {
					items = append(items, x)
				}
			}
		}
		for _, arg := range items {
			info := a.CheckExpr(arg)
			args = append(args, Argument{Type: info.Type, IsLValue: info.ValCat == LValue})
		}
		if mi.Name == nil || a.curRecord == nil || a.dependentContext() {
			continue
		}
		name := NameString(mi.Name, a.unit)
		var target types.Type
		for _, f := range a.curRecord.Fields {
			if f.Name == name {
				target = f.Type
			}
		}
		if target == nil {
			continue // a base, or a name the class does not have
		}
		if _, isArr := types.Unqualify(target).(*types.Array); isArr && mi.Braced != nil {
			// Braced-init-list on an array member.
			a.checkListInit(mi.Braced, target)
			continue
		}
		rec := types.AsRecord(types.Unqualify(target))
		if rec == nil || !hasUserConstructor(rec) || dependentArguments(args) {
			continue
		}
		// A single argument of the class itself is a copy, chosen the
		// same way; anything else is the constructor the arguments pick.
		chosen, err := a.chooseConstructor(rec, args)
		if err != nil {
			a.errorAt(mi.Pos(), fmt.Sprintf("no matching constructor for %s: %v", rec.Name, err))
			continue
		}
		if chosen != nil {
			for i := len(mi.Args); i < len(chosen.Defaults); i++ {
				if def := chosen.Defaults[i]; def != nil {
					mi.Args = append(mi.Args, def)
					a.CheckExpr(def)
				}
			}
		}
		if !chosen.Defaulted && a.info != nil {
			a.info.MemInits[mi] = chosen
		}
	}
}

// packAt is the #pragma pack ceiling in effect at a position, or zero
// where the analysis has no file to ask -- a test building scopes by hand.
func (a *Analyzer) packAt(pos ast.Tok) int64 {
	if a.file == nil {
		return 0
	}
	return a.file.PackAt(pos)
}

// checkSpecializationBody checks a class template specialization's body
// as written, in a scope of its own, and keeps nothing: see checkClassSpec.
func (a *Analyzer) checkSpecializationBody(s *ast.ClassSpec) {
	tn := s.Name.(*ast.TemplateName)
	rec := &types.Record{Tag: types.TagStruct, Name: NameString(tn.Name, a.unit), Scopes: a.curScope.Path()}
	classScope := NewScope(a.curScope, ClassScope, rec)
	classScope.Insert(&RecordSymbol{SymName: rec.Name, Record: rec, SymPos: s.Pos(), SymScope: classScope, ClassScope: classScope})

	oldScope, oldRecord, oldAccess := a.curScope, a.curRecord, a.curAccess
	a.curScope, a.curRecord = classScope, rec
	if s.Kind == token.CLASS {
		a.curAccess = types.AccessPrivate
	} else {
		a.curAccess = types.AccessPublic
	}
	for _, member := range s.Members {
		// Bodies are not checked here: a specialization's body as written
		// is checked when it is instantiated, with its parameters bound.
		// A member template's body the same -- and checking it here,
		// before the members below it were declared, found none of them.
		if fn, ok := member.(*ast.FuncDecl); ok && fn.Body != nil {
			body := fn.Body
			fn.Body = nil
			a.CheckDecl(fn)
			fn.Body = body
			continue
		}
		if td, ok := member.(*ast.TemplateDecl); ok && memberTemplateBody(td) != nil {
			fn := memberTemplateBody(td)
			body := fn.Body
			fn.Body = nil
			a.CheckDecl(td)
			fn.Body = body
			continue
		}
		a.CheckDecl(member)
	}
	rec.Complete = true
	a.deduceDefaultedComparisons(rec, s.Pos())
	a.curScope, a.curRecord, a.curAccess = oldScope, oldRecord, oldAccess
}

// declareClassTemplate is `template <bool B, class T> struct EnableIf;`:
// a class template declared and not defined, which its specializations
// hang off and a later definition completes. The usual shape of
// enable_if, whose primary is never defined at all.
func (a *Analyzer) declareClassTemplate(s *ast.ElaboratedSpec) {
	if s.Kind != token.CLASS && s.Kind != token.STRUCT && s.Kind != token.UNION {
		return
	}
	id, isIdent := s.Name.(*ast.Ident)
	if !isIdent {
		return
	}
	name := id.Text(a.unit)
	for _, existing := range a.declScope().LookupLocal(name) {
		if rs, isRec := existing.(*RecordSymbol); isRec && rs.ClassTemplate != nil {
			return // declared before; nothing new to say
		}
	}
	tag := types.TagStruct
	if s.Kind == token.CLASS {
		tag = types.TagClass
	} else if s.Kind == token.UNION {
		tag = types.TagUnion
	}
	rec := &types.Record{Tag: tag, Name: name, Scopes: a.curScope.Path()}
	rs := &RecordSymbol{SymName: name, Record: rec, SymPos: s.Pos(), SymScope: a.curScope}
	rs.ClassTemplate = &ClassTemplateInfo{
		Params:    a.curTemplateParams,
		Scope:     a.declScope(),
		Instances: map[string]*RecordSymbol{},
	}
	a.declScope().Insert(rs)
}

// declaredAlready reports whether a class symbol is one the scope holds
// already -- a template declared before its definition.
func (a *Analyzer) declaredAlready(rs *RecordSymbol) bool {
	for _, existing := range a.declScope().LookupLocal(rs.SymName) {
		if existing == rs {
			return true
		}
	}
	return false
}

// isPackDeclarator reports whether a non-type parameter's declarator is
// `T... N`, a pack.
func isPackDeclarator(d ast.Declarator) bool {
	for d != nil {
		switch node := d.(type) {
		case *ast.PackDeclarator:
			return true
		case *ast.PointerDeclarator:
			d = node.Inner
		case *ast.ParenDeclarator:
			d = node.Inner
		default:
			return false
		}
	}
	return false
}

// noteFunctionTemplate records a function template's parameters and body,
// preserving default arguments across redeclarations.
func (a *Analyzer) noteFunctionTemplate(fnSym *FuncSymbol, d *ast.FuncDecl) {
	params := a.curTemplateParams
	instances := map[string]*FuncSymbol{}
	if prev := fnSym.Template; prev != nil {
		for i, p := range params {
			if p.Default == nil && i < len(prev.Params) && prev.Params[i].Default != nil {
				p.Default = prev.Params[i].Default
			}
		}
		instances = prev.Instances
		if d == nil || d.Body == nil {
			// Nothing new to instantiate from; the defaults, if this
			// declaration adds any, went onto the parameters kept.
			for i, p := range prev.Params {
				if p.Default == nil && i < len(params) && params[i].Default != nil {
					p.Default = params[i].Default
				}
			}
			return
		}
	}
	fnSym.Template = &TemplateInfo{
		Params:    params,
		Decl:      d,
		Scope:     a.declScope(),
		Instances: instances,
	}
}

// deduceAuto deduces the type of an auto variable from its initializer.
func (a *Analyzer) deduceAuto(declared types.Type, init ExprInfo) types.Type {
	if !mentionsAuto(declared) {
		return declared
	}
	if isDependentExpr(init) {
		return &types.DependentType{Name: "auto"}
	}
	if types.IsRValueReference(declared) && init.ValCat == LValue {
		if types.Unqualify(types.RemoveReference(declared)).Kind() == types.AutoKind {
			return types.AddLValueReference(init.Type)
		}
	}
	if types.IsReference(declared) {
		return substituteAuto(declared, init.Type)
	}
	return substituteAuto(declared, types.Unqualify(types.Decay(types.RemoveReference(init.Type))))
}

// mentionsAuto reports whether a declared type still has the placeholder
// in it: `auto`, `const auto`, `auto &`, `auto *`.
func mentionsAuto(t types.Type) bool {
	switch x := types.Unqualify(t).(type) {
	case nil:
		return false
	case *types.Pointer:
		return mentionsAuto(x.Elem)
	case *types.LValueReference:
		return mentionsAuto(x.Elem)
	case *types.RValueReference:
		return mentionsAuto(x.Elem)
	}
	k := types.Unqualify(t).Kind()
	return k == types.AutoKind || k == types.DecltypeAutoKind
}

// adoptAnonymousUnion makes an anonymous union's members reachable as
// the enclosing class's: the union becomes an unnamed field, and each
// of its members is inserted into the class's scope as a symbol whose
// name lookup finds and whose storage lowering reaches through the
// unnamed field (see lookupRecordMember and fieldOffset).
func (a *Analyzer) adoptAnonymousUnion(s *ast.ClassSpec) {
	rec := a.lastAnonymousRecord
	if rec == nil {
		return
	}
	a.curRecord.Fields = append(a.curRecord.Fields, types.Field{Name: "", Type: rec, Access: a.curAccess})
	for _, f := range rec.Fields {
		if f.Name == "" {
			continue
		}
		a.curScope.Insert(&VarSymbol{SymName: f.Name, SymType: f.Type, SymPos: s.Pos(), SymScope: a.curScope})
	}
}

// declareParams registers function parameter symbols in fnScope.
func (a *Analyzer) declareParams(ft *types.Func, fnScope *Scope) []*VarSymbol {
	var params []*VarSymbol
	packs := map[string]*PackSymbol{}
	for _, p := range ft.Params {
		pVar := &VarSymbol{
			SymName:  p.Name,
			SymType:  p.Type,
			SymScope: fnScope,
			IsParam:  true,
		}
		if p.Pack {
			continue
		}
		params = append(params, pVar)
		if p.PackOf != "" {
			pack := packs[p.PackOf]
			if pack == nil {
				pack = &PackSymbol{SymName: p.PackOf, SymScope: fnScope}
				packs[p.PackOf] = pack
				fnScope.Insert(pack)
			}
			pack.Elems = append(pack.Elems, pVar)
			continue
		}
		if p.Name != "" {
			fnScope.Insert(pVar)
		}
	}
	// Preserve empty packs in scope.
	for _, p := range ft.Params {
		if p.Pack {
			if _, seen := packs[p.Name]; !seen && p.Name != "" {
				pack := &PackSymbol{SymName: p.Name, SymScope: fnScope, Open: packIn(p.Type) == nil}
				packs[p.Name] = pack
				fnScope.Insert(pack)
			}
		}
	}
	for _, name := range ft.EmptyPacks {
		if _, seen := packs[name]; !seen {
			pack := &PackSymbol{SymName: name, SymScope: fnScope}
			packs[name] = pack
			fnScope.Insert(pack)
		}
	}
	return params
}

// declareClass declares a forward-declared class.
func (a *Analyzer) declareClass(s *ast.ElaboratedSpec) {
	if s.Kind != token.CLASS && s.Kind != token.STRUCT && s.Kind != token.UNION {
		return
	}
	id, isIdent := s.Name.(*ast.Ident)
	if !isIdent {
		return
	}
	name := id.Text(a.unit)
	for _, existing := range a.declScope().LookupLocal(name) {
		if _, isRec := existing.(*RecordSymbol); isRec {
			return
		}
	}
	tag := types.TagStruct
	if s.Kind == token.CLASS {
		tag = types.TagClass
	} else if s.Kind == token.UNION {
		tag = types.TagUnion
	}
	rec := &types.Record{Tag: tag, Name: name, Scopes: a.curScope.Path()}
	rs := &RecordSymbol{SymName: name, Record: rec, SymPos: s.Pos(), SymScope: a.curScope}
	a.declScope().Insert(rs)
}

// predeclareElaborated predeclares a class named in an elaborated-type-specifier.
func (a *Analyzer) predeclareElaborated(s *ast.ElaboratedSpec) {
	if s.Kind != token.CLASS && s.Kind != token.STRUCT && s.Kind != token.UNION {
		return
	}
	id, isIdent := s.Name.(*ast.Ident)
	if !isIdent {
		return
	}
	name := id.Text(a.unit)
	for _, sym := range LookupUnqualified(a.curScope, name) {
		switch sym.(type) {
		case *RecordSymbol, *TypeSymbol, *TemplateParamSymbol:
			return
		}
	}
	tag := types.TagStruct
	if s.Kind == token.CLASS {
		tag = types.TagClass
	} else if s.Kind == token.UNION {
		tag = types.TagUnion
	}
	into := a.curScope
	for into != nil && into.Kind != NamespaceScope && into.Kind != GlobalScope {
		into = into.Parent
	}
	if into == nil {
		into = a.declScope()
	}
	rec := &types.Record{Tag: tag, Name: name, Scopes: into.Path()}
	into.Insert(&RecordSymbol{SymName: name, Record: rec, SymPos: s.Pos(), SymScope: into})
}

// predeclareParams applies predeclareElaborated to a function declarator's parameters.
func (a *Analyzer) predeclareParams(d ast.Declarator) {
	for d != nil {
		switch node := d.(type) {
		case *ast.FuncDeclarator:
			for _, p := range node.Params {
				if p == nil || p.Specs == nil {
					continue
				}
				for _, spec := range p.Specs.List {
					if es, isElab := spec.(*ast.ElaboratedSpec); isElab {
						a.predeclareElaborated(es)
					}
				}
			}
			d = node.Inner
		case *ast.PointerDeclarator:
			d = node.Inner
		case *ast.ParenDeclarator:
			d = node.Inner
		default:
			return
		}
	}
}

// hasFriend reports whether `friend` is among the specifiers.
func hasFriend(specs *ast.DeclSpecs) bool {
	if specs == nil {
		return false
	}
	for _, s := range specs.List {
		if b, isBasic := s.(*ast.BasicSpec); isBasic && b.Kind == token.FRIEND {
			return true
		}
	}
	return false
}

// checkFriendBody checks the body of a friend function defined in a class.
func (a *Analyzer) checkFriendBody(d *ast.FuncDecl) {
	into := a.curScope
	for into != nil && into.Kind != NamespaceScope && into.Kind != GlobalScope {
		into = into.Parent
	}
	saved := a.enterInstantiation(into)
	a.instantiating, a.curTemplateParams = false, saved.params
	a.definingFriend = true
	a.checkFuncDecl(d)
	a.definingFriend = false
	a.leaveInstantiation(saved)
}

// convertingConstructor resolves a non-explicit constructor for copy-initialization.
func (a *Analyzer) convertingConstructor(rec *types.Record, arg Argument, at ast.Tok) *FuncSymbol {
	var ctors []*FuncSymbol
	for _, c := range a.memberFuncs(rec, rec.Name) {
		if !c.Explicit {
			ctors = append(ctors, c)
		}
	}
	if len(ctors) == 0 {
		return nil
	}
	ndiags := len(a.diags)
	chosen, err := a.resolveAmong(ctors, nil, []Argument{arg}, at)
	if err != nil {
		a.diags = a.diags[:ndiags]
		return nil
	}
	return chosen
}

func extractDefaults(d ast.Declarator) []ast.Expr {
	fd := funcDeclaratorOf(d)
	if fd == nil {
		return nil
	}
	hasAny := false
	defaults := make([]ast.Expr, len(fd.Params))
	for i, p := range fd.Params {
		if p.Default != nil {
			defaults[i] = p.Default
			hasAny = true
		}
	}
	if !hasAny {
		return nil
	}
	return defaults
}

func (a *Analyzer) checkParamDefaults(fd *ast.FuncDeclarator, prev *FuncSymbol) {
	if fd == nil {
		return
	}
	seenDefault := false
	for i, p := range fd.Params {
		hasDef := p.Default != nil || (prev != nil && i < len(prev.Defaults) && prev.Defaults[i] != nil)
		if hasDef {
			seenDefault = true
		} else if seenDefault && !isPackParamDecl(p) {
			a.errorAt(p.Pos(), "missing default argument on parameter")
		}
	}
}

func funcDeclaratorOf(d ast.Declarator) *ast.FuncDeclarator {
	for d != nil {
		switch fd := d.(type) {
		case *ast.FuncDeclarator:
			return fd
		case *ast.ParenDeclarator:
			d = fd.Inner
		case *ast.PointerDeclarator:
			d = fd.Inner
		default:
			return nil
		}
	}
	return nil
}

// asmLabel is a declarator's GNU asm label, the string literals joined, or
// "" when it has none.
func (a *Analyzer) asmLabel(init *ast.InitDeclarator) string {
	if len(init.AsmLabel) == 0 {
		return ""
	}
	var b strings.Builder
	for _, tok := range init.AsmLabel {
		text := a.unit.Text(tok)
		if s, err := strconv.Unquote(text); err == nil {
			b.WriteString(s)
		} else {
			a.errorAt(tok, "an asm label is a plain string literal")
		}
	}
	return b.String()
}
