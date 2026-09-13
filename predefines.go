package vcx

import "github.com/vertex-language/vcx/preprocessor"

// Predefines returns the initial predefined macros for a target (architecture, OS, and dialect).
func (t Target) Predefines() []preprocessor.Predefine {
	var out []preprocessor.Predefine
	def := func(name, value string) {
		out = append(out, preprocessor.Predefine{Kind: preprocessor.PredefineDefine, Text: name + "=" + value})
	}

	switch t.Arch {
	case "amd64":
		def("__x86_64__", "1")
		def("__x86_64", "1")
		def("__amd64__", "1")
		def("__amd64", "1")
	case "arm64":
		def("__aarch64__", "1")
	case "i386":
		def("__i386__", "1")
		def("__i386", "1")
	}

	switch t.OS {
	case "linux":
		def("__linux__", "1")
		def("__gnu_linux__", "1")
		def("__unix__", "1")
		def("__ELF__", "1")
	case "macos":
		def("__APPLE__", "1")
		def("__MACH__", "1")
		if t.Arch == "arm64" {
			def("__arm64__", "1")
			def("__arm64", "1")
		}
	}

	switch t.Dialect {
	case DialectMSVC:
		for _, d := range msvcIdentity {
			def(d[0], d[1])
		}
		for _, d := range cppFeatures {
			def(d[0], d[1])
		}
		if t.Arch == "amd64" {
			def("_M_X64", "100")
			def("_M_AMD64", "100")
			def("_WIN64", "1")
		}
	default:
		for _, d := range t.gnuPredefines() {
			def(d[0], d[1])
		}
		for _, d := range cppFeatures {
			def(d[0], d[1])
		}
	}
	return out
}

// msvcIdentity defines MSVC toolset compatibility macros.
var msvcIdentity = [][2]string{
	{"_WIN32", "1"},
	{"_MSC_VER", "1944"},
	{"_MSC_FULL_VER", "194435228"},
	{"_MSC_BUILD", "0"},
	{"_MSC_EXTENSIONS", "1"},
	{"_MSVC_LANG", "202400L"},
	{"_MSVC_TRADITIONAL", "0"},
	{"_MSVC_WARNING_LEVEL", "1L"},
	{"_MSVC_EXECUTION_CHARACTER_SET", "1252"},
	{"_MSVC_CONSTEXPR_ATTRIBUTE", "1"},
	{"_MT", "1"},
	{"_CPPRTTI", "1"},
	{"_INTEGRAL_MAX_BITS", "64"},
	{"_NATIVE_WCHAR_T_DEFINED", "1"},
	{"_WCHAR_T_DEFINED", "1"},
	{"_NATIVE_NULLPTR_SUPPORTED", "1"},
	{"_BUILTIN_LAUNDER_SUPPORTED", "1"},
	{"_HAS_CHAR16_T_LANGUAGE_SUPPORT", "1"},
	{"_IS_ASSIGNABLE_NOCHECK_SUPPORTED", "1"},
	{"_CONSTEXPR_CHAR_TRAITS_SUPPORTED", "1"},
	{"_NOEXCEPT_TYPES_SUPPORTED", "1"},
	{"_CRT_USE_BUILTIN_OFFSETOF", "1"},
	{"__BOOL_DEFINED", "1"},
	{"__STDCPP_DEFAULT_NEW_ALIGNMENT__", "16ull"},
	{"__STDCPP_THREADS__", "1"},
}

// cppFeatures defines C++ feature-test macros supported by vcx.
var cppFeatures = [][2]string{
	{"__cpp_aggregate_bases", "201603L"},
	{"__cpp_aggregate_nsdmi", "201304L"},
	{"__cpp_aggregate_paren_init", "201902L"},
	{"__cpp_alias_templates", "200704L"},
	{"__cpp_aligned_new", "201606L"},
	{"__cpp_attributes", "200809L"},
	{"__cpp_binary_literals", "201304L"},
	{"__cpp_capture_star_this", "201603L"},
	{"__cpp_char8_t", "202207L"},
	{"__cpp_concepts", "202002L"},
	{"__cpp_conditional_explicit", "201806L"},
	{"__cpp_consteval", "201811L"},
	{"__cpp_constexpr", "202002L"},
	{"__cpp_constexpr_dynamic_alloc", "201907L"},
	{"__cpp_constinit", "201907L"},
	{"__cpp_decltype", "200707L"},
	{"__cpp_decltype_auto", "201304L"},
	{"__cpp_deduction_guides", "201907L"},
	{"__cpp_delegating_constructors", "200604L"},
	{"__cpp_designated_initializers", "201707L"},
	{"__cpp_enumerator_attributes", "201411L"},
	{"__cpp_fold_expressions", "201603L"},
	{"__cpp_generic_lambdas", "201707L"},
	{"__cpp_guaranteed_copy_elision", "201606L"},
	{"__cpp_hex_float", "201603L"},
	{"__cpp_if_consteval", "202106L"},
	{"__cpp_if_constexpr", "201606L"},
	{"__cpp_impl_coroutine", "201902L"},
	{"__cpp_impl_destroying_delete", "201806L"},
	{"__cpp_impl_three_way_comparison", "201907L"},
	{"__cpp_inheriting_constructors", "201511L"},
	{"__cpp_init_captures", "201803L"},
	{"__cpp_initializer_lists", "200806L"},
	{"__cpp_inline_variables", "201606L"},
	{"__cpp_lambdas", "200907L"},
	{"__cpp_modules", "201907L"},
	{"__cpp_multidimensional_subscript", "202211L"},
	{"__cpp_namespace_attributes", "201411L"},
	{"__cpp_noexcept_function_type", "201510L"},
	{"__cpp_nontype_template_args", "201911L"},
	{"__cpp_nontype_template_parameter_auto", "201606L"},
	{"__cpp_nsdmi", "200809L"},
	{"__cpp_range_based_for", "201603L"},
	{"__cpp_raw_strings", "200710L"},
	{"__cpp_ref_qualifiers", "200710L"},
	{"__cpp_return_type_deduction", "201304L"},
	{"__cpp_rtti", "199711L"},
	{"__cpp_rvalue_references", "200610L"},
	{"__cpp_size_t_suffix", "202011L"},
	{"__cpp_sized_deallocation", "201309L"},
	{"__cpp_static_assert", "201411L"},
	{"__cpp_static_call_operator", "202207L"},
	{"__cpp_structured_bindings", "201606L"},
	{"__cpp_template_template_args", "201611L"},
	{"__cpp_threadsafe_static_init", "200806L"},
	{"__cpp_unicode_characters", "200704L"},
	{"__cpp_unicode_literals", "200710L"},
	{"__cpp_user_defined_literals", "200809L"},
	{"__cpp_using_enum", "201907L"},
	{"__cpp_variable_templates", "201304L"},
	{"__cpp_variadic_templates", "200704L"},
	{"__cpp_variadic_using", "201611L"},
}
