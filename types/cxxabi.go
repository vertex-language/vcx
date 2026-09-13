package types

// CXXABI is the C++ ABI a target's objects follow: how classes are laid
// out, how their tables and structors are shaped, how members are pointed
// at. It is one axis of a target and nothing else -- not its data model,
// which the sizes on Model say, and not its dialect, which is the
// preprocessor's and the parser's business.
//
// The kinds are clang's, because clang is a compiler that already had to
// be every one of them at once: generic Itanium, the ARM variant of it that
// AArch64 Linux follows, Apple's arm64 variant of that, and Microsoft's.
// Code that needs to know asks one of the questions below rather than
// comparing kinds, so that a target added later is a row here and not a
// search through the lowering for every place that assumed two ABIs.
type CXXABI uint8

const (
	// ItaniumGeneric is the Itanium C++ ABI as x86-64 and i386 SysV use it:
	// Linux, macOS on Intel, bare-metal ELF.
	ItaniumGeneric CXXABI = iota

	// ItaniumAArch64 is ARM's C++ ABI for AArch64: Itanium with member
	// function pointers and guard variables done ARM's way.
	ItaniumAArch64

	// ItaniumAppleARM64 is Apple's arm64 ABI: ARM's, and in addition
	// structors that return `this`, array cookies that keep the element
	// size, and C++11's rule for when tail padding is reused.
	ItaniumAppleARM64

	// Microsoft is MSVC's.
	Microsoft
)

func (a CXXABI) String() string {
	switch a {
	case ItaniumAArch64:
		return "itanium-aarch64"
	case ItaniumAppleARM64:
		return "itanium-apple-arm64"
	case Microsoft:
		return "microsoft"
	}
	return "itanium"
}

// IsMicrosoft is the MSVC family: its layout, tables, vbtables, name
// scheme and calling conventions for classes.
func (a CXXABI) IsMicrosoft() bool { return a == Microsoft }

// IsItanium is every other kind here.
func (a CXXABI) IsItanium() bool { return a != Microsoft }

// TailPaddingPOD11 is whether a class's tail padding is reusable unless it
// is a POD in C++11's sense, rather than C++ TR1's. Apple's arm64 only.
func (a CXXABI) TailPaddingPOD11() bool { return a == ItaniumAppleARM64 }

// ConstructorsReturnThis is whether a constructor returns the object it
// constructed: Microsoft's convention, and ARM's as Apple inherited it.
func (a CXXABI) ConstructorsReturnThis() bool {
	return a == Microsoft || a == ItaniumAppleARM64
}

// DestructorsReturnThis is the same for a non-deleting destructor, which
// only the ARM-derived Apple ABI does.
func (a CXXABI) DestructorsReturnThis() bool { return a == ItaniumAppleARM64 }

// ArrayCookieHasElementSize is ARM's array cookie as Apple keeps it: the
// element size before the count, sixteen bytes where Itanium keeps eight.
func (a CXXABI) ArrayCookieHasElementSize() bool { return a == ItaniumAppleARM64 }

// ARMMemberFunctionPointers is ARM's member function pointer, which marks a
// virtual function in the adjustment rather than in the pointer.
func (a CXXABI) ARMMemberFunctionPointers() bool {
	return a == ItaniumAArch64 || a == ItaniumAppleARM64
}

// ResultAfterThis is Microsoft's placement of a member function's hidden
// result pointer: after `this`, where Itanium puts it first.
func (a CXXABI) ResultAfterThis() bool { return a == Microsoft }

// CalleeDestroysParameters is Microsoft's rule that the called function
// destroys a class parameter passed by value; under Itanium the caller does.
func (a CXXABI) CalleeDestroysParameters() bool { return a == Microsoft }
