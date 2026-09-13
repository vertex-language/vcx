package vcx

import (
	"fmt"
	"strconv"

	"github.com/vertex-language/vcx/types"
)

// Predefined macros for the GNU dialect (gcc/clang compatibility).

// claimedClang is the clang release vcx presents itself as.
const (
	claimedClangMajor = 19
	claimedClangMinor = 1
	claimedClangPatch = 0
)

func (t Target) gnuPredefines() [][2]string {
	m := types.ModelForTarget(t.Arch, t.OS)
	var out [][2]string
	def := func(name, value string) { out = append(out, [2]string{name, value}) }

	version := fmt.Sprintf("%d.%d.%d", claimedClangMajor, claimedClangMinor, claimedClangPatch)
	def("__clang__", "1")
	def("__clang_major__", strconv.Itoa(claimedClangMajor))
	def("__clang_minor__", strconv.Itoa(claimedClangMinor))
	def("__clang_patchlevel__", strconv.Itoa(claimedClangPatch))
	def("__clang_version__", `"`+version+` (vcx)"`)
	def("__clang_literal_encoding__", `"UTF-8"`)
	def("__VERSION__", `"vcx, compatible with Clang `+version+`"`)
	def("__GNUC__", "4")
	def("__GNUC_MINOR__", "2")
	def("__GNUC_PATCHLEVEL__", "1")
	def("__GNUG__", "4")
	def("__GXX_ABI_VERSION", "1002")
	def("__GXX_EXPERIMENTAL_CXX0X__", "1")
	def("__GXX_WEAK__", "1")
	def("__GXX_RTTI", "1")
	def("__STDC__", "1")
	def("__STDC_UTF_16__", "1")
	def("__STDC_UTF_32__", "1")
	def("__STDCPP_THREADS__", "1")
	def("__NO_INLINE__", "1")
	def("__FINITE_MATH_ONLY__", "0")

	t.gnuDataModel(m, def)
	gnuFloats(m, def)
	t.gnuArch(def)

	switch t.OS {
	case "linux":
		def("__linux", "1")
		def("__unix", "1")
		// clang defines it for C++ on Linux because libstdc++ needs it.
		def("_GNU_SOURCE", "1")
	case "macos":
		t.gnuDarwin(def)
	}
	return out
}

// intSpelling is how gcc and clang print an integer type in a macro.
var intSpelling = map[types.Kind]string{
	types.SChar:     "signed char",
	types.UChar:     "unsigned char",
	types.Short:     "short",
	types.UShort:    "unsigned short",
	types.Int:       "int",
	types.UInt:      "unsigned int",
	types.Long:      "long int",
	types.ULong:     "long unsigned int",
	types.LongLong:  "long long int",
	types.ULongLong: "long long unsigned int",
}

// intSuffix is the literal suffix a constant of the type is written with.
var intSuffix = map[types.Kind]string{
	types.UInt: "U", types.Long: "L", types.ULong: "UL", types.LongLong: "LL", types.ULongLong: "ULL",
}

// intLength is the printf length modifier for the type.
var intLength = map[types.Kind]string{
	types.SChar: "hh", types.UChar: "hh", types.Short: "h", types.UShort: "h",
	types.Long: "l", types.ULong: "l", types.LongLong: "ll", types.ULongLong: "ll",
}

func unsignedKind(k types.Kind) bool {
	switch k {
	case types.UChar, types.UShort, types.UInt, types.ULong, types.ULongLong:
		return true
	}
	return false
}

func toUnsigned(k types.Kind) types.Kind {
	switch k {
	case types.SChar:
		return types.UChar
	case types.Short:
		return types.UShort
	case types.Int:
		return types.UInt
	case types.Long:
		return types.ULong
	case types.LongLong:
		return types.ULongLong
	}
	return k
}

// gnuTypes defines target-specific typedef choices for standard integer types.
type gnuTypes struct {
	size, intmax, int64, intptr, wchar, wint types.Kind
}

func (t Target) gnuTypeChoices(m types.Model) gnuTypes {
	g := gnuTypes{size: types.ULong, intmax: types.Long, int64: types.Long, intptr: types.Long, wchar: m.WCharKind, wint: types.Int}
	if m.SizePtr == m.SizeInt {
		g.size, g.intptr, g.intmax, g.int64 = types.UInt, types.Int, types.LongLong, types.LongLong
	}
	if t.OS == "macos" {
		g.int64 = types.LongLong
	}
	if t.OS == "linux" {
		g.wint = types.UInt
	}
	return g
}

func (m typeSizer) bits(k types.Kind) int64 {
	n, _ := m.Model.Sizeof(types.Typ(k))
	return n * 8
}

type typeSizer struct{ types.Model }

func (t Target) gnuDataModel(m types.Model, def func(name, value string)) {
	sz := typeSizer{m}
	g := t.gnuTypeChoices(m)

	max := func(k types.Kind) string {
		b := sz.bits(k)
		var v string
		if unsignedKind(k) {
			v = strconv.FormatUint(uint64(1)<<uint(b)-1, 10)
			if b == 64 {
				v = "18446744073709551615"
			}
		} else {
			v = strconv.FormatInt(int64(uint64(1)<<uint(b-1)-1), 10)
		}
		return v + intSuffix[k]
	}
	named := func(prefix string, k types.Kind, width bool) {
		def("__"+prefix+"_TYPE__", intSpelling[k])
		def("__"+prefix+"_MAX__", max(k))
		if width {
			def("__"+prefix+"_WIDTH__", strconv.FormatInt(sz.bits(k), 10))
		}
		if unsignedKind(k) {
			for _, c := range []string{"o", "u", "x", "X"} {
				def("__"+prefix+"_FMT"+c+"__", `"`+intLength[k]+c+`"`)
			}
		} else {
			for _, c := range []string{"d", "i"} {
				def("__"+prefix+"_FMT"+c+"__", `"`+intLength[k]+c+`"`)
			}
		}
	}
	constant := func(prefix string, k types.Kind) {
		sfx := intSuffix[k]
		def("__"+prefix+"_C_SUFFIX__", sfx)
		if sfx == "" {
			def("__"+prefix+"_C(c)", "c")
		} else {
			def("__"+prefix+"_C(c)", "c##"+sfx)
		}
	}

	def("__CHAR_BIT__", "8")
	def("__BOOL_WIDTH__", "1")
	def("__SCHAR_MAX__", max(types.SChar))
	def("__SHRT_MAX__", max(types.Short))
	def("__SHRT_WIDTH__", "16")
	def("__INT_MAX__", max(types.Int))
	def("__INT_WIDTH__", strconv.FormatInt(sz.bits(types.Int), 10))
	def("__LONG_MAX__", max(types.Long))
	def("__LONG_WIDTH__", strconv.FormatInt(sz.bits(types.Long), 10))
	def("__LONG_LONG_MAX__", max(types.LongLong))
	def("__LLONG_WIDTH__", "64")
	def("__POINTER_WIDTH__", strconv.FormatInt(m.SizePtr*8, 10))
	if !m.CharSigned {
		def("__CHAR_UNSIGNED__", "1")
	}

	for _, s := range [][2]any{
		{"SHORT", m.SizeShort}, {"INT", m.SizeInt}, {"LONG", m.SizeLong}, {"LONG_LONG", m.SizeLongLong},
		{"POINTER", m.SizePtr}, {"FLOAT", m.SizeFloat}, {"DOUBLE", m.SizeDouble}, {"LONG_DOUBLE", m.SizeLongDouble},
	} {
		def("__SIZEOF_"+s[0].(string)+"__", strconv.FormatInt(s[1].(int64), 10))
	}
	def("__SIZEOF_SIZE_T__", strconv.FormatInt(sz.bits(g.size)/8, 10))
	def("__SIZEOF_PTRDIFF_T__", strconv.FormatInt(sz.bits(g.intptr)/8, 10))
	def("__SIZEOF_WCHAR_T__", strconv.FormatInt(sz.bits(g.wchar)/8, 10))
	def("__SIZEOF_WINT_T__", strconv.FormatInt(sz.bits(g.wint)/8, 10))
	if m.SizePtr == 8 {
		def("__SIZEOF_INT128__", "16")
	}

	switch {
	case m.SizePtr == 8 && m.SizeLong == 8:
		def("__LP64__", "1")
		def("_LP64", "1")
	case m.SizePtr == 4:
		def("__ILP32__", "1")
		def("_ILP32", "1")
	}
	def("__ORDER_LITTLE_ENDIAN__", "1234")
	def("__ORDER_BIG_ENDIAN__", "4321")
	def("__ORDER_PDP_ENDIAN__", "3412")
	def("__BYTE_ORDER__", "__ORDER_LITTLE_ENDIAN__")
	def("__LITTLE_ENDIAN__", "1")

	named("SIZE", g.size, true)
	named("PTRDIFF", g.intptr, true)
	named("INTPTR", g.intptr, true)
	named("UINTPTR", toUnsigned(g.intptr), true)
	named("INTMAX", g.intmax, true)
	named("UINTMAX", toUnsigned(g.intmax), true)
	constant("INTMAX", g.intmax)
	constant("UINTMAX", toUnsigned(g.intmax))
	def("__WCHAR_TYPE__", intSpelling[g.wchar])
	def("__WCHAR_MAX__", max(g.wchar))
	def("__WCHAR_WIDTH__", strconv.FormatInt(sz.bits(g.wchar), 10))
	def("__WINT_TYPE__", intSpelling[g.wint])
	def("__WINT_MAX__", max(g.wint))
	def("__WINT_WIDTH__", strconv.FormatInt(sz.bits(g.wint), 10))
	if unsignedKind(g.wint) {
		def("__WINT_UNSIGNED__", "1")
	}
	def("__SIG_ATOMIC_MAX__", max(types.Int))
	def("__SIG_ATOMIC_WIDTH__", strconv.FormatInt(sz.bits(types.Int), 10))
	def("__CHAR16_TYPE__", "unsigned short")
	def("__CHAR32_TYPE__", "unsigned int")

	exact := map[int]types.Kind{8: types.SChar, 16: types.Short, 32: types.Int, 64: g.int64}
	for _, bits := range []int{8, 16, 32, 64} {
		k := exact[bits]
		n := strconv.Itoa(bits)
		for _, pair := range [][2]types.Kind{{k, k}, {toUnsigned(k), toUnsigned(k)}} {
			kk := pair[0]
			u := ""
			if unsignedKind(kk) {
				u = "U"
			}
			def("__"+u+"INT"+n+"_TYPE__", intSpelling[kk])
			def("__"+u+"INT"+n+"_MAX__", max(kk))
			fmts := []string{"d", "i"}
			if u != "" {
				fmts = []string{"o", "u", "x", "X"}
			}
			for _, c := range fmts {
				def("__"+u+"INT"+n+"_FMT"+c+"__", `"`+intLength[kk]+c+`"`)
			}
			constant(u+"INT"+n, kk)
			for _, kind := range []string{"LEAST", "FAST"} {
				named(u+"INT_"+kind+n, kk, u == "")
			}
		}
	}

	align := int64(16)
	if t.OS == "macos" && t.Arch == "arm64" {
		align = 8
	}
	if t.Arch == "i386" {
		align = 16
	}
	def("__BIGGEST_ALIGNMENT__", strconv.FormatInt(align, 10))
	def("__STDCPP_DEFAULT_NEW_ALIGNMENT__", "16"+intSuffix[g.size])

	prefix := ""
	if t.Container == ContainerMachO {
		prefix = "_"
	}
	def("__USER_LABEL_PREFIX__", prefix)
	if t.Arch != "arm64" || t.OS == "macos" {
		def("__REGISTER_PREFIX__", "")
	}

	for _, name := range []string{"BOOL", "CHAR", "CHAR16_T", "CHAR32_T", "WCHAR_T", "SHORT", "INT", "LONG", "LLONG", "POINTER", "CHAR8_T"} {
		def("__GCC_ATOMIC_"+name+"_LOCK_FREE", "2")
		def("__CLANG_ATOMIC_"+name+"_LOCK_FREE", "2")
	}
	def("__GCC_ATOMIC_TEST_AND_SET_TRUEVAL", "1")
	for _, n := range []string{"1", "2", "4", "8"} {
		def("__GCC_HAVE_SYNC_COMPARE_AND_SWAP_"+n, "1")
	}
}

// gnuFloats is the float, double and long double characteristics <float.h>
// and <limits> are built from.
func gnuFloats(m types.Model, def func(name, value string)) {
	type format struct {
		mantDig, dig, decimalDig, minExp, minTenExp, maxExp, maxTenExp int
		denormMin, epsilon, max, min, suffix                           string
	}
	single := format{24, 6, 9, -125, -37, 128, 38, "1.40129846e-45", "1.19209290e-7", "3.40282347e+38", "1.17549435e-38", "F"}
	double := format{53, 15, 17, -1021, -307, 1024, 308, "4.9406564584124654e-324", "2.2204460492503131e-16", "1.7976931348623157e+308", "2.2250738585072014e-308", ""}
	x87 := format{64, 18, 21, -16381, -4931, 16384, 4932, "3.64519953188247460253e-4951", "1.08420217248550443401e-19", "1.18973149535723176502e+4932", "3.36210314311209350626e-4932", "L"}
	quad := format{113, 33, 36, -16381, -4931, 16384, 4932, "6.47517511943802511092443895822764655e-4966", "1.92592994438723585305597794258492732e-34", "1.18973149535723176508575932662800702e+4932", "3.36210314311209350626267781732175260e-4932", "L"}

	emit := func(prefix string, f format) {
		def("__"+prefix+"_MANT_DIG__", strconv.Itoa(f.mantDig))
		def("__"+prefix+"_DIG__", strconv.Itoa(f.dig))
		def("__"+prefix+"_DECIMAL_DIG__", strconv.Itoa(f.decimalDig))
		def("__"+prefix+"_MIN_EXP__", "("+strconv.Itoa(f.minExp)+")")
		def("__"+prefix+"_MIN_10_EXP__", "("+strconv.Itoa(f.minTenExp)+")")
		def("__"+prefix+"_MAX_EXP__", strconv.Itoa(f.maxExp))
		def("__"+prefix+"_MAX_10_EXP__", strconv.Itoa(f.maxTenExp))
		def("__"+prefix+"_DENORM_MIN__", f.denormMin+f.suffix)
		def("__"+prefix+"_EPSILON__", f.epsilon+f.suffix)
		def("__"+prefix+"_MAX__", f.max+f.suffix)
		def("__"+prefix+"_MIN__", f.min+f.suffix)
		def("__"+prefix+"_HAS_DENORM__", "1")
		def("__"+prefix+"_HAS_INFINITY__", "1")
		def("__"+prefix+"_HAS_QUIET_NAN__", "1")
	}
	emit("FLT", single)
	emit("DBL", double)
	long := double
	long.suffix = "L"
	switch m.SizeLongDouble {
	case 16:
		if m.CharSigned {
			long = x87 // x86-64: the 80-bit format in sixteen bytes
		} else {
			long = quad // AArch64 Linux: binary128
		}
	case 12:
		long = x87
	}
	emit("LDBL", long)
	def("__FLT_RADIX__", "2")
	def("__DECIMAL_DIG__", "__LDBL_DECIMAL_DIG__")
}

// gnuArch is what clang adds for the architecture beyond the identity
// Predefines already gave it.
func (t Target) gnuArch(def func(name, value string)) {
	switch t.Arch {
	case "arm64":
		def("__AARCH64EL__", "1")
		def("__ARM_64BIT_STATE", "1")
		def("__ARM_ARCH", "8")
		def("__ARM_PCS_AAPCS64", "1")
		def("__ARM_FP", "0xE")
		def("__ARM_ALIGN_MAX_STACK_PWR", "4")
		def("__AARCH64_CMODEL_SMALL__", "1")
	case "amd64":
		def("__SSE__", "1")
		def("__SSE2__", "1")
		def("__SSE_MATH__", "1")
		def("__SSE2_MATH__", "1")
		def("__MMX__", "1")
		def("__FXSR__", "1")
		def("__code_model_small__", "1")
	}
}

// gnuDarwin is the Apple platform's own macros: the deployment target the
// SDK's availability machinery reads, and the TARGET_OS_ answers clang
// defines outright.
func (t Target) gnuDarwin(def func(name, value string)) {
	def("__APPLE_CC__", "6000")
	def("__private_extern__", "extern")
	def("__PIC__", "2")
	def("__pic__", "2")
	def("__DYNAMIC__", "1")
	def("__NO_MATH_ERRNO__", "1")
	minOS := macOSMinimum(t)
	var major, minor int
	fmt.Sscanf(minOS, "%d.%d", &major, &minor)
	v := strconv.Itoa(major*10000 + minor*100)
	def("__ENVIRONMENT_MAC_OS_X_VERSION_MIN_REQUIRED__", v)
	def("__ENVIRONMENT_OS_VERSION_MIN_REQUIRED__", v)
	for _, name := range []string{"TARGET_OS_MAC", "TARGET_OS_OSX", "TARGET_OS_UNIX"} {
		if name == "TARGET_OS_UNIX" {
			def(name, "0")
			continue
		}
		def(name, "1")
	}
	for _, name := range []string{
		"TARGET_OS_IPHONE", "TARGET_OS_IOS", "TARGET_OS_TV", "TARGET_OS_WATCH", "TARGET_OS_VISION",
		"TARGET_OS_XR", "TARGET_OS_MACCATALYST", "TARGET_OS_UIKITFORMAC", "TARGET_OS_IOSMAC",
		"TARGET_OS_SIMULATOR", "TARGET_IPHONE_SIMULATOR", "TARGET_OS_EMBEDDED", "TARGET_OS_NANO",
		"TARGET_OS_BRIDGE", "TARGET_OS_DRIVERKIT", "TARGET_OS_LINUX", "TARGET_OS_WIN32",
		"TARGET_OS_WINDOWS", "TARGET_OS_UEFI",
	} {
		def(name, "0")
	}
	arrow := "0"
	if t.Arch == "arm64" {
		arrow = "1"
	}
	def("TARGET_OS_ARROW", arrow)
}
