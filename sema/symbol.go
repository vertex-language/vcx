package sema

import (
	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/types"
)

// StorageClass indicates variable linkage and duration.
type StorageClass uint8

const (
	StorageAuto StorageClass = iota
	StorageStatic
	StorageExtern
	StorageThreadLocal
)

// Symbol represents a declared named entity.
type Symbol interface {
	Name() string
	Type() types.Type
	Pos() ast.Tok
	Scope() *Scope
}

// VarSymbol represents a variable, parameter, or static data member.
type VarSymbol struct {
	SymName   string
	SymType   types.Type
	SymPos    ast.Tok
	SymScope  *Scope
	Storage   StorageClass
	Constexpr bool
	Consteval bool
	IsParam   bool
	Init      ast.Expr

	// BracedInit is the `= { ... }` or `{ ... }` initializer, when the
	// declaration had one instead of an expression.
	BracedInit *ast.InitList

	// InClass is the class of a static data member.
	InClass *types.Record

	// ExternC is an object declared with C language linkage (unmangled).
	ExternC bool

	// Inline marks an inline or constexpr variable.
	Inline bool

	// Defined marks a declaration that is a definition.
	Defined bool

	// KnownValue is the value of a non-type template parameter in an
	// instantiation, where there is no initializer to evaluate.
	KnownValue    int64
	HasKnownValue bool

	// Binding is set on a name introduced by a structured binding.
	Binding *BindingInfo

	// Template is set on a variable template (e.g. `template <class T> constexpr bool is_void_v = ...`).
	Template     *VarTemplate
	TemplateArgs []types.TemplateArg
}

// A VarTemplate is a variable template's declaration and the instances
// made from it so far.
type VarTemplate struct {
	Params    []*TemplateParamSymbol
	Decl      *ast.SimpleDecl
	Scope     *Scope
	Instances map[string]*VarSymbol

	// Explicit are the full specializations by argument list, and
	// Partials the partial ones, each with the parameters its pattern is
	// written in terms of.
	Explicit map[string]*ast.SimpleDecl
	Partials []*VarPartial
}

// A VarPartial is `template <class T> constexpr bool v<T *> = true;`.
type VarPartial struct {
	Params []*TemplateParamSymbol
	Args   []types.TemplateArg
	Decl   *ast.SimpleDecl
	Scope  *Scope
}

func (v *VarSymbol) Name() string     { return v.SymName }
func (v *VarSymbol) Type() types.Type { return v.SymType }
func (v *VarSymbol) Pos() ast.Tok     { return v.SymPos }
func (v *VarSymbol) Scope() *Scope    { return v.SymScope }

// FuncSymbol represents a function, member function, constructor, or operator.
type FuncSymbol struct {
	// Decl is the definition this symbol was made from, when it was made
	// from one. A constructor's mem-initializer-list lives on it, and
	// lowering has to run that list before the body.
	Decl *ast.FuncDecl

	SymName     string
	FuncType    *types.Func
	SymPos      ast.Tok
	SymScope    *Scope
	Params      []*VarSymbol
	Defaults    []ast.Expr
	Body        *ast.CompoundStmt
	Inline      bool
	Constexpr   bool
	Consteval   bool
	Virtual     bool
	PureVirtual bool
	Static      bool
	Defaulted   bool
	Deleted     bool
	Explicit    bool
	Friend      bool
	InClass     *types.Record

	// Access is the member's access when InClass is set.
	Access types.Access

	// ExternC indicates C language linkage (unmangled name).
	ExternC bool

	// LinkName, when set, is the name the linker sees in place of SymName,
	// before the container adds its prefix: a library builtin
	// `__builtin_fabs` is a call to `fabs`.
	LinkName string

	// AsmLabel is GNU's `__asm("_realpath$DARWIN_EXTSN")`: the object
	// file's symbol exactly, prefix and all.
	AsmLabel string

	// Constraints are the function's requires-clauses that must hold for
	// the function to be a candidate. ConstraintScope is the evaluation scope.
	Constraints     []ast.Expr
	ConstraintScope *Scope

	Template     *TemplateInfo
	TemplateArgs []types.TemplateArg
	TemplateOf   *FuncSymbol

	// Method is the class's record of this member.
	Method *types.Method
}

func (f *FuncSymbol) Name() string     { return f.SymName }
func (f *FuncSymbol) Type() types.Type { return f.FuncType }
func (f *FuncSymbol) Pos() ast.Tok     { return f.SymPos }
func (f *FuncSymbol) Scope() *Scope    { return f.SymScope }

// PackSymbol represents a function parameter pack in its function's scope.
type PackSymbol struct {
	SymName  string
	SymPos   ast.Tok
	SymScope *Scope
	Elems    []*VarSymbol

	// Open indicates the pack is not yet bound to concrete arguments.
	Open bool
}

func (p *PackSymbol) Name() string     { return p.SymName }
func (p *PackSymbol) Type() types.Type { return &types.Pack{} }
func (p *PackSymbol) Pos() ast.Tok     { return p.SymPos }
func (p *PackSymbol) Scope() *Scope    { return p.SymScope }

// DependentSymbol represents an unresolved qualified name whose qualifier
// depends on an uninstantiated template parameter.
type DependentSymbol struct {
	SymName string
	SymPos  ast.Tok
}

func (d *DependentSymbol) Name() string     { return d.SymName }
func (d *DependentSymbol) Type() types.Type { return &types.DependentType{Name: d.SymName} }
func (d *DependentSymbol) Pos() ast.Tok     { return d.SymPos }
func (d *DependentSymbol) Scope() *Scope    { return nil }

// TypeSymbol represents a typedef or type alias (using T = ...).
type TypeSymbol struct {
	// Alias is set when the alias is a template.
	Alias *AliasTemplate

	SymName  string
	SymType  types.Type
	SymPos   ast.Tok
	SymScope *Scope
}

func (t *TypeSymbol) Name() string     { return t.SymName }
func (t *TypeSymbol) Type() types.Type { return t.SymType }
func (t *TypeSymbol) Pos() ast.Tok     { return t.SymPos }
func (t *TypeSymbol) Scope() *Scope    { return t.SymScope }

// RecordSymbol represents a class, struct, or union definition.
type RecordSymbol struct {
	SymName    string
	Record     *types.Record
	SymPos     ast.Tok
	SymScope   *Scope
	ClassScope *Scope

	ClassTemplate *ClassTemplateInfo
	TemplateOf    *RecordSymbol

	// Spec is the class-specifier that defined the class.
	Spec *ast.ClassSpec
}

// AliasTemplate represents an alias template (template<...> using ...).
type AliasTemplate struct {
	Params []*TemplateParamSymbol
	Type   *ast.TypeId
	Scope  *Scope

	// Builtin is set on compiler-provided templates such as __make_integer_seq.
	Builtin string
}

func (r *RecordSymbol) Name() string     { return r.SymName }
func (r *RecordSymbol) Type() types.Type { return r.Record }
func (r *RecordSymbol) Pos() ast.Tok     { return r.SymPos }
func (r *RecordSymbol) Scope() *Scope    { return r.SymScope }

// EnumSymbol represents an enum definition.
type EnumSymbol struct {
	SymName  string
	Enum     *types.Enum
	SymPos   ast.Tok
	SymScope *Scope
}

func (e *EnumSymbol) Name() string     { return e.SymName }
func (e *EnumSymbol) Type() types.Type { return e.Enum }
func (e *EnumSymbol) Pos() ast.Tok     { return e.SymPos }
func (e *EnumSymbol) Scope() *Scope    { return e.SymScope }

// EnumeratorSymbol represents an enumerator constant.
type EnumeratorSymbol struct {
	SymName  string
	Enum     *types.Enum
	Val      int64
	SymPos   ast.Tok
	SymScope *Scope
}

func (e *EnumeratorSymbol) Name() string     { return e.SymName }
func (e *EnumeratorSymbol) Type() types.Type { return e.Enum }
func (e *EnumeratorSymbol) Pos() ast.Tok     { return e.SymPos }
func (e *EnumeratorSymbol) Scope() *Scope    { return e.SymScope }

// NamespaceSymbol represents a namespace.
type NamespaceSymbol struct {
	SymName    string
	SymPos     ast.Tok
	SymScope   *Scope
	InnerScope *Scope
}

func (n *NamespaceSymbol) Name() string     { return n.SymName }
func (n *NamespaceSymbol) Type() types.Type { return nil }
func (n *NamespaceSymbol) Pos() ast.Tok     { return n.SymPos }
func (n *NamespaceSymbol) Scope() *Scope    { return n.SymScope }

// TemplateParamSymbol represents a template parameter.
type TemplateParamSymbol struct {
	SymName string
	Index   int
	Depth   int
	IsType  bool
	IsPack  bool

	// IsTemplate marks a template template parameter (template <...> class T).
	IsTemplate bool
	SymType    types.Type
	SymPos     ast.Tok
	SymScope   *Scope

	// Default is the default template argument (ast.TypeId or ast.Expr).
	Default ast.Node

	// Decl is a non-type parameter's declaration, used when rebuilding
	// types dependent on preceding parameters.
	Decl *ast.ParamDecl
}

func (tp *TemplateParamSymbol) Name() string     { return tp.SymName }
func (tp *TemplateParamSymbol) Type() types.Type { return tp.SymType }
func (tp *TemplateParamSymbol) Pos() ast.Tok     { return tp.SymPos }
func (tp *TemplateParamSymbol) Scope() *Scope    { return tp.SymScope }

// ConceptSymbol represents a C++20/23 concept declaration.
type ConceptSymbol struct {
	SymName    string
	SymPos     ast.Tok
	SymScope   *Scope
	Params     []*TemplateParamSymbol
	Constraint ast.Expr
}

func (c *ConceptSymbol) Name() string     { return c.SymName }
func (c *ConceptSymbol) Type() types.Type { return nil }
func (c *ConceptSymbol) Pos() ast.Tok     { return c.SymPos }
func (c *ConceptSymbol) Scope() *Scope    { return c.SymScope }

// TemplateSymbol represents a template declaration (class, function, variable, or alias template).
type TemplateSymbol struct {
	SymName  string
	SymPos   ast.Tok
	SymScope *Scope
	Params   []*TemplateParamSymbol
	Decl     ast.Decl
}

func (t *TemplateSymbol) Name() string     { return t.SymName }
func (t *TemplateSymbol) Type() types.Type { return nil }
func (t *TemplateSymbol) Pos() ast.Tok     { return t.SymPos }
func (t *TemplateSymbol) Scope() *Scope    { return t.SymScope }
