package vcx

import "github.com/vertex-language/vcx/preprocessor"

// objcPredefines are the macros an Objective-C++ unit has beyond a C++
// one's, as clang -x objective-c++ -fobjc-arc defines them. ARC is always
// on: a .mm file is compiled the way SwiftPM compiles one.
func objcPredefines() []preprocessor.Predefine {
	var out []preprocessor.Predefine
	def := func(text string) {
		out = append(out, preprocessor.Predefine{Kind: preprocessor.PredefineDefine, Text: text})
	}
	undef := func(name string) {
		out = append(out, preprocessor.Predefine{Kind: preprocessor.PredefineUndef, Text: name})
	}
	def("__OBJC__=1")
	def("__OBJC2__=1")
	def("__NEXT_RUNTIME__=1")
	def("OBJC_NEW_PROPERTIES=1")
	def("OBJC_ZEROCOST_EXCEPTIONS=1")
	def("__BLOCKS__=1")
	def("__block=__attribute__((__blocks__(byref)))")
	// The ownership qualifiers are attributes, which a C++ unit spells as
	// nothing (or GC's weak).
	for _, q := range []struct{ name, owner string }{
		{"__strong", "strong"},
		{"__weak", "weak"},
		{"__autoreleasing", "autoreleasing"},
		{"__unsafe_unretained", "none"},
	} {
		undef(q.name)
		def(q.name + "=__attribute__((objc_ownership(" + q.owner + ")))")
	}
	def("IBOutlet=__attribute__((iboutlet))")
	def("IBOutletCollection(ClassName)=__attribute__((iboutletcollection(ClassName)))")
	def("IBAction=void)__attribute__((ibaction)")
	def("IBInspectable=")
	def("IB_DESIGNABLE=")
	return out
}

// objcFeatures are what __has_feature answers yes to in an Objective-C++
// unit, as clang -fobjc-arc answers.
var objcFeatures = []string{
	"objc_arc", "objc_arc_weak", "objc_arc_fields", "objc_arr", "objc_weak_class",
	"objc_instancetype", "objc_array_literals", "objc_dictionary_literals",
	"objc_subscripting", "objc_boxed_expressions", "objc_boxed_nsvalue_expressions",
	"objc_fixed_enum", "objc_default_synthesize_properties", "objc_nonfragile_abi",
	"objc_bool", "objc_class_property", "objc_generics", "objc_generics_variance",
	"objc_kindof", "objc_protocol_qualifier_mangling",
	"blocks", "nullability", "nullability_on_arrays",
}
