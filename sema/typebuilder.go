package sema

import (
	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/constexpr"
	"github.com/vertex-language/vcx/token"
	"github.com/vertex-language/vcx/types"
)

// DeclSpecInfo records the semantic interpretation of a decl-specifier-seq.
type DeclSpecInfo struct {
	Type      types.Type
	Storage   StorageClass
	Inline    bool
	Constexpr bool
	Consteval bool
	Virtual   bool
	Explicit  bool
	Friend    bool
	Typedef   bool
	Quals     types.Qual
	Trailing  bool

	// Unresolved names an unqualified type-name in the decl-specifier-seq
	// that lookup did not find, with the token it was written at. The type
	// falls back to int so the rest of the declaration still builds; the
	// caller reports it, because BuildDeclSpecs has no diagnostic sink and
	// is called from places -- template argument construction, trailing
	// return types -- where a dependent name legitimately resolves to
	// nothing.
	Unresolved    string
	UnresolvedPos ast.Tok
}

// BuildDeclSpecs evaluates DeclSpecs and produces the base type and specifier flags.
func BuildDeclSpecs(specs *ast.DeclSpecs, scope *Scope, u ast.Unit) DeclSpecInfo {
	var info DeclSpecInfo
	if specs == nil {
		info.Type = types.Typ(types.Int)
		return info
	}

	var (
		sawVoid     bool
		sawBool     bool
		sawChar     bool
		sawChar8    bool
		sawChar16   bool
		sawChar32   bool
		sawWChar    bool
		sawInt      bool
		sawShort    bool
		sawLong     int
		sawSigned   bool
		sawUnsigned bool
		sawFloat    bool
		sawDouble   bool
		sawAuto     bool
		customType  types.Type
	)

	for _, spec := range specs.List {
		switch s := spec.(type) {
		case *ast.BasicSpec:
			switch s.Kind {
			case token.VOID:
				sawVoid = true
			case token.BOOL:
				sawBool = true
			case token.CHAR:
				sawChar = true
			case token.CHAR8_T:
				sawChar8 = true
			case token.CHAR16_T:
				sawChar16 = true
			case token.CHAR32_T:
				sawChar32 = true
			case token.WCHAR_T:
				sawWChar = true
			case token.INT:
				sawInt = true
			case token.SHORT:
				sawShort = true
			case token.LONG:
				sawLong++
			case token.INT8:
				// Microsoft's sized integers are the standard ones by
				// another name: __int8 is char, __int16 short, __int32 int
				// and __int64 long long, and `unsigned __int64` is the
				// unsigned one.
				sawChar = true
			case token.INT16:
				sawShort = true
			case token.INT32:
				sawInt = true
			case token.INT64:
				sawLong = 2
			case token.SIGNED:
				sawSigned = true
			case token.UNSIGNED:
				sawUnsigned = true
			case token.FLOAT:
				sawFloat = true
			case token.DOUBLE:
				sawDouble = true
			case token.AUTO:
				sawAuto = true
			case token.CONST:
				info.Quals |= types.QConst
			case token.VOLATILE:
				info.Quals |= types.QVolatile
			case token.STATIC:
				info.Storage = StorageStatic
			case token.EXTERN:
				info.Storage = StorageExtern
			case token.THREAD_LOCAL:
				info.Storage = StorageThreadLocal
			case token.INLINE:
				info.Inline = true
			case token.CONSTEXPR:
				info.Constexpr = true
			case token.CONSTEVAL:
				info.Consteval = true
			case token.VIRTUAL:
				info.Virtual = true
			case token.EXPLICIT:
				info.Explicit = true
			case token.FRIEND:
				info.Friend = true
			case token.TYPEDEF:
				info.Typedef = true
			}

		case *ast.NamedTypeSpec:
			var syms []Symbol
			var tmplName *ast.TemplateName
			name := s.Name
			if pn, isPack := name.(*ast.PackName); isPack {
				// `Ts... vs` -- the parser put the ellipsis on the type;
				// the type is the pack's, and the parameter's declarator
				// carries the expansion (see isPackParam).
				name = pn.Name
			}
			if tn, ok := name.(*ast.TemplateName); ok {
				tmplName = tn
			}
			if qn, ok := name.(*ast.QualifiedName); ok {
				// `std::vector<int>` -- the template-id is the last
				// component, and the qualifiers say where its template
				// lives.
				if tn, isTemplate := qn.Name.(*ast.TemplateName); isTemplate {
					tmplName = tn
				}
				syms = ResolveQualifiedName(qn, scope, nil, u)
			} else {
				nameStr := NameString(name, u)
				syms = LookupUnqualified(scope, nameStr)
			}
			if len(syms) == 0 {
				if _, qualified := name.(*ast.QualifiedName); !qualified && info.Unresolved == "" {
					info.Unresolved = NameString(name, u)
					info.UnresolvedPos = name.Pos()
				}
			}
			if sym := firstTypeSymbol(syms); sym != nil {
				customType = sym.Type()
				if tmplName != nil {
					args := templateArgs(tmplName, scope, u)
					// A specialization named with concrete arguments is
					// instantiated, when an analysis is running to do it
					// and the arguments say nothing about a template
					// parameter still open. Otherwise -- inside a template
					// being checked as written -- it stays the dependent
					// spelling.
					instantiated := false
					if concrete(args) {
						switch ts := sym.(type) {
						case *RecordSymbol:
							if ts.TemplateOf != nil {
								ts = ts.TemplateOf
							}
							if ts.ClassTemplate != nil {
								if inst := scope.instantiate(); inst != nil {
									if t := inst(ts, args, tmplName.Pos()); t != nil {
										customType, instantiated = t, true
									}
								}
							}
						case *TypeSymbol:
							if ts.Alias != nil {
								if inst := scope.root().AliasInstantiate; inst != nil {
									if t := inst(ts, args, tmplName.Pos()); t != nil {
										customType, instantiated = t, true
									}
								}
							}
						}
					}
					if !instantiated {
						customType = &types.TemplateSpecialization{
							Name: NameString(tmplName.Name, u),
							Args: args,
							Type: customType,
						}
					}
				}
			} else {
				customType = &types.DependentType{Name: NameString(name, u)}
			}

		case *ast.ElaboratedSpec:
			// Elaborated type specifier (e.g. `struct B *p` or `enum E`).
			if s.Name == nil {
				break
			}
			var syms []Symbol
			if qn, ok := s.Name.(*ast.QualifiedName); ok {
				syms = ResolveQualifiedName(qn, scope, nil, u)
			} else {
				syms = LookupUnqualified(scope, NameString(s.Name, u))
			}
			for _, sym := range syms {
				switch t := sym.(type) {
				case *RecordSymbol:
					customType = t.Record
				case *EnumSymbol:
					customType = t.Enum
				case *TemplateParamSymbol:
					customType = t.SymType
				default:
					continue
				}
				break
			}
			if customType == nil {
				customType = &types.DependentType{Name: NameString(s.Name, u)}
				if _, qualified := s.Name.(*ast.QualifiedName); !qualified && info.Unresolved == "" {
					info.Unresolved = NameString(s.Name, u)
					info.UnresolvedPos = s.Name.Pos()
				}
			}
		case *ast.ClassSpec:
			if s.Name == nil {
				if rec := scope.anonymousRecord(s); rec != nil {
					customType = rec
				}
			}
			if s.Name != nil {
				var syms []Symbol
				if qn, ok := s.Name.(*ast.QualifiedName); ok {
					syms = ResolveQualifiedName(qn, scope, nil, u)
				} else {
					syms = LookupUnqualified(scope, NameString(s.Name, u))
				}
				for _, sym := range syms {
					if rs, ok := sym.(*RecordSymbol); ok {
						customType = rs.Record
						break
					}
				}
			}

		case *ast.EnumSpec:
			if s.Name != nil {
				var syms []Symbol
				if qn, ok := s.Name.(*ast.QualifiedName); ok {
					syms = ResolveQualifiedName(qn, scope, nil, u)
				} else {
					syms = LookupUnqualified(scope, NameString(s.Name, u))
				}
				for _, sym := range syms {
					if es, ok := sym.(*EnumSymbol); ok {
						customType = es.Enum
						break
					}
				}
			}

		case *ast.DecltypeSpec:
			switch {
			case s.Auto.IsValid():
				customType = types.Typ(types.DecltypeAutoKind)
			case scope != nil && scope.decltype() != nil:
				customType = scope.decltype()(s.X, scope)
			default:
				// No analysis is running to type the operand, so it stays
				// a placeholder, as `auto` does until its initializer.
				customType = types.Typ(types.AutoKind)
			}

		case *ast.ConstrainedAutoSpec:
			customType = types.Typ(types.AutoKind)

		case *ast.TypeTransformSpec:
			if s.Arg == nil {
				break
			}
			argInfo := BuildDeclSpecs(s.Arg.Specs, scope, u)
			arg := BuildDeclarator(s.Arg.Decl, argInfo.Type, scope, u)
			if t, ok := types.ApplyTransform(u.Text(s.Name), arg, isDependentType(arg)); ok && t != nil {
				customType = t
			}
		}
	}

	if customType != nil {
		info.Type = types.Qualify(customType, info.Quals)
		return info
	}

	// Determine primitive type from specifier multiset
	var base types.Type
	switch {
	case sawVoid:
		base = types.Typ(types.Void)
	case sawBool:
		base = types.Typ(types.Bool)
	case sawChar:
		if sawSigned {
			base = types.Typ(types.SChar)
		} else if sawUnsigned {
			base = types.Typ(types.UChar)
		} else {
			base = types.Typ(types.Char)
		}
	case sawChar8:
		base = types.Typ(types.Char8)
	case sawChar16:
		base = types.Typ(types.Char16)
	case sawChar32:
		base = types.Typ(types.Char32)
	case sawWChar:
		base = types.Typ(types.WChar)
	case sawFloat:
		base = types.Typ(types.Float)
	case sawDouble:
		if sawLong > 0 {
			base = types.Typ(types.LongDouble)
		} else {
			base = types.Typ(types.Double)
		}
	case sawLong > 0:
		if sawLong == 1 {
			if sawUnsigned {
				base = types.Typ(types.ULong)
			} else {
				base = types.Typ(types.Long)
			}
		} else {
			if sawUnsigned {
				base = types.Typ(types.ULongLong)
			} else {
				base = types.Typ(types.LongLong)
			}
		}
	case sawShort:
		if sawUnsigned {
			base = types.Typ(types.UShort)
		} else {
			base = types.Typ(types.Short)
		}
	case sawUnsigned:
		base = types.Typ(types.UInt)
	case sawSigned:
		base = types.Typ(types.Int)
	case sawInt:
		base = types.Typ(types.Int)
	case sawAuto:
		base = types.Typ(types.AutoKind)
	default:
		base = types.Typ(types.Int)
	}

	info.Type = types.Qualify(base, info.Quals)
	return info
}

// BuildDeclarator wraps baseType with pointers, references, arrays, and functions from d.
// resolveClassName is the class named to the left of the `::*` in a
// pointer-to-member declarator.
//
// The parser leaves it as a Name because it is written where a
// nested-name-specifier goes and may be qualified; what the type needs is
// the class it denotes. A name that resolves to nothing yields nil, and the
// member pointer is then sized the most general way rather than the
// smallest -- see Model.memberPointerSize.
func resolveClassName(n ast.Name, scope *Scope, u ast.Unit) types.Type {
	if n == nil {
		return nil
	}
	// The `C::` of a pointer-to-member parses as a QualifiedName whose final
	// component is the `*`, so the class is the qualifier rather than the
	// name. A bare identifier is the class itself.
	if qn, ok := n.(*ast.QualifiedName); ok && len(qn.Qual) > 0 {
		n = qn.Qual[len(qn.Qual)-1]
	}
	if sym := firstTypeSymbol(LookupUnqualified(scope, NameString(n, u))); sym != nil {
		return sym.Type()
	}
	return nil
}

// firstTypeSymbol picks the first symbol suitable for a type context out of a lookup.
func firstTypeSymbol(syms []Symbol) Symbol {
	for _, sym := range syms {
		switch sym.(type) {
		case *TypeSymbol, *RecordSymbol, *EnumSymbol, *TemplateParamSymbol, *TemplateSymbol:
			return sym
		}
	}
	if len(syms) > 0 {
		// Nothing in the set is a type. Returning the first keeps whatever
		// diagnostic the caller would have produced before this filter.
		return syms[0]
	}
	return nil
}

func BuildDeclarator(d ast.Declarator, baseType types.Type, scope *Scope, u ast.Unit) types.Type {
	if d == nil || baseType == nil {
		return baseType
	}

	switch decl := d.(type) {
	case *ast.NameDeclarator:
		return baseType

	case *ast.PointerDeclarator:
		// The ptr-operator applies to what is written to its left.
		var quals types.Qual
		for _, q := range decl.Quals {
			if b, ok := q.(*ast.BasicSpec); ok {
				if b.Kind == token.CONST {
					quals |= types.QConst
				} else if b.Kind == token.VOLATILE {
					quals |= types.QVolatile
				}
			}
		}

		var wrapped types.Type
		switch decl.Kind {
		case token.MUL:
			// Member pointer: `int C::*` is a pointer to a member of class C.
			if decl.Class != nil {
				cls := resolveClassName(decl.Class, scope, u)
				wrapped = types.Qualify(&types.MemberPointer{Class: cls, Elem: baseType}, quals)
				break
			}
			wrapped = types.Qualify(&types.Pointer{Elem: baseType}, quals)
		case token.AND:
			wrapped = types.AddLValueReference(baseType)
		case token.LAND:
			wrapped = types.AddRValueReference(baseType)
		default:
			wrapped = baseType
		}
		return BuildDeclarator(decl.Inner, wrapped, scope, u)

	case *ast.ArrayDeclarator:
		// The array bound is evaluated as a constant expression.
		incomplete := decl.Size == nil
		var arr types.Type
		if incomplete {
			arr = &types.Array{Elem: baseType, Incomplete: true}
		} else if val, ok := evalArrayBound(decl.Size, scope, u); ok {
			arr = &types.Array{Elem: baseType, Len: val}
		} else {
			name := "?"
			if id, isIdent := decl.Size.(*ast.Ident); isIdent {
				name = id.Text(u)
			}
			arr = &types.Array{Elem: baseType, DepLen: name}
		}
		return BuildDeclarator(decl.Inner, arr, scope, u)

	case *ast.FuncDeclarator:
		ret := baseType
		var params []types.Param
		var emptyPacks []string
		for _, p := range decl.Params {
			pInfo := BuildDeclSpecs(p.Specs, scope, u)
			pType := BuildDeclarator(p.Decl, pInfo.Type, scope, u)
			name := ""
			if p.Decl != nil && p.Decl.DeclName() != nil {
				name = NameString(p.Decl.DeclName(), u)
			}
			if isPackParamDecl(p) {
				// Pack expansion parameter: one parameter per element of bound pack.
				if pack := packIn(pType); pack != nil {
					if len(pack.Elems) == 0 {
						emptyPacks = append(emptyPacks, name)
					}
					for _, elem := range pack.Elems {
						params = append(params, types.Param{
							Name:   name,
							Type:   substitutePack(pType, elem.Type),
							PackOf: name,
						})
					}
					continue
				}
				params = append(params, types.Param{Name: name, Type: pType, Pack: true})
				continue
			}
			params = append(params, types.Param{
				Name:       name,
				Type:       pType,
				HasDefault: p.Default != nil || p.Assign.IsValid(),
			})
		}
		// Treat `f(void)` as an empty parameter list.
		if len(params) == 1 && params[0].Name == "" && params[0].Type != nil && types.IsVoid(types.Unqualify(params[0].Type)) {
			params = nil
		}

		if decl.Trailing != nil && decl.Trailing.Type != nil {
			// Trailing return type: built in scope containing parameter declarations.
			trailScope := NewScope(scope, BlockScope, nil)
			for _, p := range params {
				if p.Name != "" {
					trailScope.Insert(&VarSymbol{SymName: p.Name, SymType: p.Type, SymScope: trailScope, IsParam: true})
				}
			}
			trailInfo := BuildDeclSpecs(decl.Trailing.Type.Specs, trailScope, u)
			ret = BuildDeclarator(decl.Trailing.Type.Decl, trailInfo.Type, trailScope, u)
		}

		var quals types.Qual
		for _, q := range decl.Quals {
			if b, ok := q.(*ast.BasicSpec); ok {
				if b.Kind == token.CONST {
					quals |= types.QConst
				} else if b.Kind == token.VOLATILE {
					quals |= types.QVolatile
				}
			}
		}

		refQual := types.RefQualNone
		if decl.RefKind == token.AND {
			refQual = types.RefQualLValue
		} else if decl.RefKind == token.LAND {
			refQual = types.RefQualRValue
		}

		fn := &types.Func{
			Ret:        ret,
			Params:     params,
			Variadic:   decl.Vararg.IsValid(),
			Quals:      quals,
			RefQual:    refQual,
			Noexcept:   decl.Noexcept != nil,
			EmptyPacks: emptyPacks,
		}
		return BuildDeclarator(decl.Inner, fn, scope, u)

	case *ast.ParenDeclarator:
		return BuildDeclarator(decl.Inner, baseType, scope, u)

	case *ast.BitfieldDeclarator:
		return BuildDeclarator(decl.Inner, baseType, scope, u)

	case *ast.PackDeclarator:
		return BuildDeclarator(decl.Inner, baseType, scope, u)
	}

	return baseType
}

// NameString converts an ast.Name to its string representation using Unit.
func NameString(name ast.Name, u ast.Unit) string {
	if name == nil {
		return ""
	}
	switch n := name.(type) {
	case *ast.Ident:
		return n.Text(u)
	case *ast.QualifiedName:
		var s string
		if n.Global.IsValid() {
			s = "::"
		}
		for _, q := range n.Qual {
			s += NameString(q, u) + "::"
		}
		if n.Name != nil {
			s += NameString(n.Name, u)
		}
		return s
	case *ast.TemplateName:
		return NameString(n.Name, u)
	case *ast.OperatorName:
		// The spelling is the operator, and for the three bracketed forms
		// that is a token pair rather than the single token OpPos names.
		// Reading OpPos alone gives "operator(" and "operator[", which are
		// not the names anything looks up.
		switch {
		case n.Op == token.LPAREN:
			return "operator()"
		case n.Op == token.LBRACK:
			return "operator[]"
		case n.Op == token.NEW || n.Op == token.DELETE:
			// The allocation functions are spelled with a space, the way
			// the standard and the manglers' tables spell them.
			if n.Array {
				return "operator " + u.Text(n.OpPos) + "[]"
			}
			return "operator " + u.Text(n.OpPos)
		case n.Array:
			return "operator" + u.Text(n.OpPos) + "[]"
		}
		return "operator" + u.Text(n.OpPos)

	case *ast.DestructorName:
		return "~" + NameString(n.Name, u)
	case *ast.ConversionName:
		// A conversion function is named for the type it converts to (e.g. operator bool).
		name := "operator"
		for t := n.Type.Pos(); t < n.Type.End(); t++ {
			name += " " + u.Text(t)
		}
		return name
	}
	return ""
}

// templateArgs reads a template-argument-list.
func templateArgs(tn *ast.TemplateName, scope *Scope, u ast.Unit) []types.TemplateArg {
	// Never nil: `X<>` has an argument list, an empty one, and callers
	// read nil as "no list".
	args := []types.TemplateArg{}
	eval := scope.root().EvalConst
	value := func(e ast.Expr) types.TemplateArg {
		// A non-type parameter still open -- N in a partial
		// specialization's pattern `X<T, N>` -- is kept as the
		// parameter, so that matching against the pattern can bind it.
		if id, isIdent := e.(*ast.Ident); isIdent {
			for _, sym := range LookupUnqualified(scope, id.Text(u)) {
				if tp, isParam := sym.(*TemplateParamSymbol); isParam && !tp.IsType {
					return types.TemplateArg{IsType: true, Type: &types.TemplateParam{Name: tp.SymName, Index: tp.Index}}
				}
				break
			}
		}
		if eval != nil {
			if n, ok := eval(e, scope); ok {
				return types.TemplateArg{IsType: false, Val: n}
			}
		}
		// Not evaluable here: dependent, or no evaluator. A dependent
		// type stands in so the specialization reads as dependent.
		return types.TemplateArg{IsType: true, Type: &types.DependentType{Name: "<value>"}}
	}
	for _, argNode := range tn.Args {
		switch arg := argNode.(type) {
		case *ast.TypeId:
			// When a type-id argument resolves to a value, treat it as a value argument.
			if name := bareTypeName(arg); name != nil {
				if pn, isPack := name.(*ast.PackName); isPack {
					args = append(args, packArgs(pn, scope, u)...)
					continue
				}
				if e, isExpr := name.(ast.Expr); isExpr && namesAValue(lookupName(name, scope, u)) {
					args = append(args, value(e))
					continue
				}
			}
			argT := BuildDeclarator(arg.Decl, BuildDeclSpecs(arg.Specs, scope, u).Type, scope, u)
			args = append(args, types.TemplateArg{IsType: true, Type: argT})
		case *ast.Ident, *ast.QualifiedName:
			// Argument for a template template parameter.
			if rs, isRec := firstTypeSymbol(lookupName(arg, scope, u)).(*RecordSymbol); isRec && rs.ClassTemplate != nil && rs.Record != nil && rs.TemplateOf == nil {
				args = append(args, types.TemplateArg{IsType: true, Type: &types.TemplateRef{Name: rs.Record.Name, Primary: rs.Record}})
				continue
			}
			if sym := firstTypeSymbol(lookupName(arg, scope, u)); sym != nil && sym.Type() != nil {
				if _, isType := sym.(*TypeSymbol); isType || isTypeLike(sym) {
					args = append(args, types.TemplateArg{IsType: true, Type: sym.Type()})
					continue
				}
			}
			args = append(args, value(arg.(ast.Expr)))
		case *ast.PackName:
			args = append(args, packArgs(arg, scope, u)...)
		case *ast.PackExpansion:
			// Pack expansion contributes one argument per element.
			if expand := scope.root().ExpandValues; expand != nil {
				if vals, ok := expand(arg, scope); ok {
					for _, v := range vals {
						args = append(args, types.TemplateArg{Val: v})
					}
					continue
				}
			}
			args = append(args, types.TemplateArg{IsType: true, Type: &types.DependentType{Name: "<pack>"}})
		case ast.Expr:
			args = append(args, value(arg))
		default:
			args = append(args, types.TemplateArg{IsType: false})
		}
	}
	return args
}

// isTypeLike reports whether a symbol names a type.
func isTypeLike(sym Symbol) bool {
	switch s := sym.(type) {
	case *TypeSymbol, *RecordSymbol, *EnumSymbol:
		return true
	case *TemplateParamSymbol:
		return s.IsType
	}
	return false
}

// concrete reports whether every argument is settled: a type that is not
// dependent, or a value.
func concrete(args []types.TemplateArg) bool {
	for _, a := range args {
		if a.IsType && (a.Type == nil || isDependentType(a.Type)) {
			return false
		}
	}
	return true
}

// lookupName resolves a name node the way a type-specifier's is resolved.
func lookupName(n ast.Node, scope *Scope, u ast.Unit) []Symbol {
	switch n := n.(type) {
	case *ast.QualifiedName:
		return ResolveQualifiedName(n, scope, nil, u)
	case *ast.Ident:
		return LookupUnqualified(scope, n.Text(u))
	}
	return nil
}

// evalArrayBound is an array declarator's bound as a number, when it is
// one: through the analysis's evaluator where one is installed, so that
// a constant of the scope resolves, and the bare one otherwise.
func evalArrayBound(size ast.Expr, scope *Scope, u ast.Unit) (int64, bool) {
	if scope != nil {
		if eval := scope.root().EvalConst; eval != nil {
			n, ok := eval(size, scope)
			return n, ok && n >= 0
		}
	}
	ctx := constexpr.NewContext(u, types.LP64())
	val, err := ctx.EvalInt(size)
	return val, err == nil && val >= 0
}

// bareTypeName is the name a type-id consists of and nothing else -- no
// declarator, no other specifier -- or nil.
func bareTypeName(id *ast.TypeId) ast.Name {
	if id == nil || id.Specs == nil || len(id.Specs.List) != 1 {
		return nil
	}
	if id.Decl != nil {
		if nd, isLeaf := id.Decl.(*ast.NameDeclarator); !isLeaf || nd.Name != nil {
			return nil
		}
	}
	ns, isNamed := id.Specs.List[0].(*ast.NamedTypeSpec)
	if !isNamed || ns.Typename.IsValid() {
		return nil
	}
	return ns.Name
}

// namesAValue reports whether a name resolved to an object, an
// enumerator or a function rather than a type.
func namesAValue(syms []Symbol) bool {
	if len(syms) == 0 {
		return false
	}
	switch syms[0].(type) {
	case *VarSymbol, *EnumeratorSymbol, *FuncSymbol:
		return true
	}
	return false
}

// packArgs returns template arguments from a pack expansion.
func packArgs(pn *ast.PackName, scope *Scope, u ast.Unit) []types.TemplateArg {
	if sym := firstTypeSymbol(lookupName(pn.Name, scope, u)); sym != nil && sym.Type() != nil && isTypeLike(sym) {
		if pack, isPack := sym.Type().(*types.Pack); isPack {
			return pack.Elems
		}
		return []types.TemplateArg{{IsType: true, Type: sym.Type()}}
	}
	return []types.TemplateArg{{IsType: true, Type: &types.DependentType{Name: NameString(pn.Name, u)}}}
}

// packIn finds the bound pack a type mentions -- the `Ts` of `Ts &&`
// once Ts names its arguments -- or nil.
func packIn(t types.Type) *types.Pack {
	switch x := t.(type) {
	case *types.Pack:
		return x
	case *types.Qualified:
		return packIn(x.T)
	case *types.Pointer:
		return packIn(x.Elem)
	case *types.LValueReference:
		return packIn(x.Elem)
	case *types.RValueReference:
		return packIn(x.Elem)
	case *types.Array:
		return packIn(x.Elem)
	}
	return nil
}

// substitutePack is the pattern with one element in the pack's place.
func substitutePack(t types.Type, elem types.Type) types.Type {
	switch x := t.(type) {
	case *types.Pack:
		return elem
	case *types.Qualified:
		return types.Qualify(substitutePack(x.T, elem), x.Q)
	case *types.Pointer:
		return &types.Pointer{Elem: substitutePack(x.Elem, elem)}
	case *types.LValueReference:
		return types.AddLValueReference(substitutePack(x.Elem, elem))
	case *types.RValueReference:
		return types.AddRValueReference(substitutePack(x.Elem, elem))
	case *types.Array:
		return &types.Array{Elem: substitutePack(x.Elem, elem), Len: x.Len, Incomplete: x.Incomplete, DepLen: x.DepLen}
	}
	return t
}

// isPackParamDecl reports whether a parameter-declaration is a pack
// expansion: `Ts &&... vs`, whose declarator carries the ellipsis, or
// `Ts... vs`, where the parser left it on the type's name.
func isPackParamDecl(p *ast.ParamDecl) bool {
	if isPackDeclarator(p.Decl) {
		return true
	}
	if p.Specs != nil {
		for _, spec := range p.Specs.List {
			if ns, isNamed := spec.(*ast.NamedTypeSpec); isNamed {
				if _, isPack := ns.Name.(*ast.PackName); isPack {
					return true
				}
			}
		}
	}
	return false
}
