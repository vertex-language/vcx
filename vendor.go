package vcx

import (
	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/preprocessor"
	"github.com/vertex-language/vcx/sema"
)

// GNU-dialect features, builtins, and attributes supported by vcx.

// gnuFeatures are the __has_feature names vcx implements.
var gnuFeatures = []string{
	"cxx_access_control_sfinae", "cxx_aggregate_nsdmi", "cxx_alias_templates", "cxx_alignas",
	"cxx_alignof", "cxx_attributes", "cxx_auto_type", "cxx_binary_literals", "cxx_constexpr",
	"cxx_contextual_conversions", "cxx_decltype", "cxx_decltype_auto",
	"cxx_default_function_template_args", "cxx_defaulted_functions", "cxx_delegating_constructors",
	"cxx_deleted_functions", "cxx_digit_separators", "cxx_explicit_conversions",
	"cxx_generalized_initializers", "cxx_generic_lambdas", "cxx_inheriting_constructors",
	"cxx_init_captures", "cxx_inline_namespaces", "cxx_lambdas", "cxx_local_type_template_args",
	"cxx_noexcept", "cxx_nonstatic_member_init", "cxx_nullptr", "cxx_override_control",
	"cxx_range_for", "cxx_raw_string_literals", "cxx_reference_qualified_functions",
	"cxx_relaxed_constexpr", "cxx_return_type_deduction", "cxx_rtti", "cxx_rvalue_references",
	"cxx_static_assert", "cxx_strong_enums", "cxx_trailing_return", "cxx_unicode_literals",
	"cxx_unrestricted_unions", "cxx_user_literals", "cxx_variable_templates", "cxx_variadic_templates",
}

// gnuBuiltins are the __has_builtin names vcx's analysis answers: the
// builtin functions and the type traits written as keywords.
var gnuBuiltins = []string{
	"__builtin_addressof", "__builtin_assume", "__builtin_expect", "__builtin_is_constant_evaluated",
	"__builtin_offsetof", "__builtin_unreachable", "__make_integer_seq",
	"__has_nothrow_assign", "__has_nothrow_constructor", "__has_nothrow_copy", "__has_trivial_assign",
	"__has_trivial_constructor", "__has_trivial_copy", "__has_trivial_destructor",
	"__has_unique_object_representations", "__has_virtual_destructor",
	"__is_abstract", "__is_aggregate", "__is_arithmetic", "__is_array", "__is_assignable",
	"__is_base_of", "__is_bounded_array", "__is_class", "__is_compound", "__is_const",
	"__is_constructible", "__is_convertible", "__is_convertible_to", "__is_destructible", "__is_empty",
	"__is_enum", "__is_final", "__is_floating_point", "__is_function", "__is_fundamental",
	"__is_integral", "__is_layout_compatible", "__is_literal_type", "__is_lvalue_reference",
	"__is_member_function_pointer", "__is_member_object_pointer", "__is_member_pointer",
	"__is_nothrow_assignable", "__is_nothrow_constructible", "__is_nothrow_convertible",
	"__is_nothrow_destructible", "__is_nullptr", "__is_object", "__is_pod", "__is_pointer",
	"__is_pointer_interconvertible_base_of", "__is_polymorphic", "__is_reference",
	"__is_rvalue_reference", "__is_same", "__is_scalar", "__is_scoped_enum", "__is_signed",
	"__is_standard_layout", "__is_trivial", "__is_trivially_assignable", "__is_trivially_constructible",
	"__is_trivially_copyable", "__is_trivially_destructible", "__is_unbounded_array", "__is_union",
	"__is_unsigned", "__is_void", "__is_volatile", "__reference_binds_to_temporary",
	"__reference_constructs_from_temporary", "__underlying_type",
}

// gnuAttributes are the __has_attribute names supported by vcx.
var gnuAttributes = map[string]int{
	"deprecated": 1, "unavailable": 1, "warn_unused_result": 1, "fallthrough": 1, "format": 1,
	"noinline": 1, "always_inline": 1, "cold": 1, "malloc": 1, "alloc_size": 1, "alloc_align": 1,
	"returns_nonnull": 1, "noescape": 1, "not_tail_called": 1, "disable_tail_calls": 1,
	"nodebug": 1, "standalone_debug": 1, "exclude_from_explicit_instantiation": 1,
	"no_sanitize": 1, "no_thread_safety_analysis": 1, "acquire_capability": 1, "requires_capability": 1,
	"require_constant_initialization": 1, "preferred_name": 1, "flag_enum": 1, "enum_extensibility": 1,
	"swift_attr": 1, "unsafe_buffer_usage": 1,
}

// gnuVendor is the preprocessor's Vendor for a GNU-dialect target.
func (t Target) gnuVendor() *preprocessor.Vendor {
	v := &preprocessor.Vendor{
		Features:   set(gnuFeatures),
		Extensions: map[string]bool{},
		Builtins:   set(gnuBuiltins),
		Attributes: gnuAttributes,
	}
	// The type transformations are the parser's list, so that what the
	// operator promises and what a type specifier accepts are one set.
	for name := range ast.TypeTransforms {
		v.Builtins[name] = true
	}
	for _, name := range append(sema.ExpressionBuiltins(), sema.LibraryBuiltins()...) {
		v.Builtins[name] = true
	}
	switch t.Arch {
	case "arm64":
		v.Arch = []string{"aarch64", "arm64"}
	case "amd64":
		v.Arch = []string{"x86_64", "amd64"}
	case "i386":
		v.Arch = []string{"i386", "x86"}
	}
	switch t.OS {
	case "macos":
		v.VendorName = []string{"apple"}
		v.OS = []string{"macos", "macosx", "darwin"}
	case "linux":
		v.VendorName = []string{"unknown"}
		v.OS = []string{"linux"}
		v.Environment = []string{"gnu"}
	default:
		v.VendorName = []string{"unknown"}
		v.OS = []string{"none"}
	}
	return v
}

func set(names []string) map[string]bool {
	m := make(map[string]bool, len(names))
	for _, n := range names {
		m[n] = true
	}
	return m
}
