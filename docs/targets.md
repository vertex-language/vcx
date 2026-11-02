# Targets and ABIs

vcx is one C++ compiler. The MSVC and GNU families are peers, and a target chooses between them on three independent axes, the way clang's triple does: the **C++ ABI** (how classes, tables, structors and names are laid out), the **dialect** (which compiler vcx presents itself as to headers, and which vendor extensions those headers expect), and the **data model** (the sizes of the basic types).

| Target | Architecture | OS | Container | C++ ABI | Dialect |
| --- | --- | --- | --- | --- | --- |
| `x86_64-linux` | x86_64 | Linux | ELF | Itanium | GNU |
| `aarch64-linux` | AArch64 | Linux | ELF | Itanium (ARM) | GNU |
| `x86_64-windows` | x86_64 | Windows | PE/COFF | Microsoft | MSVC |
| `x86_64-macos` | x86_64 | macOS | Mach-O | Itanium | GNU |
| `aarch64-macos` | AArch64 | macOS | Mach-O | Itanium (Apple arm64) | GNU |
| `x86_64-elf` | x86_64 | Bare-metal | ELF | Itanium | GNU |
| `i386-elf` | i386 | Bare-metal | ELF | Itanium | GNU |
| `nvptx64-cuda` | NVPTX | CUDA device | PTX | Itanium | GNU |
| `amdgcn-hsa` | AMDGCN | HSA device | ELF code object | Itanium | GNU |
| `air64-apple` | AIR | Apple GPU | `.metallib` | Itanium | GNU |

The device targets are what the device pass of a CUDA, HIP or Metal unit is compiled for; they are not named with `-target`. The pass keeps the host's data model (a CUDA `long` is 32 bits on Windows, as nvcc has it) and headers, adds `__CUDA_ARCH__` or `__HIP_DEVICE_COMPILE__` and the device's builtins, and mangles with the Itanium ABI whatever the host uses, which is the name the host registers the kernel by. A `.metal` file has no host: its one pass is `air64-apple`'s, with MSL's LP64 data model and none of a CPU's macros or headers, and its kernels keep their names unmangled, as Metal looks them up.

Target selection handles:
- **Data models**: LP64 (Linux/macOS), LLP64 (Windows), ILP32 (i386), with Apple arm64's 64-bit `long double`.
- **C++ ABI** (`types.CXXABI`): Microsoft layout, vftables and vbtables; Itanium layout with tail-padding reuse and vtable groups, and its ARM and Apple arm64 variants -- structors that return `this`, 16-byte array cookies, C++11 POD rules for tail padding.
- **Name mangling**: Itanium (`_Z...`) or Microsoft (`?...`).
- **Dialect**: cl's identity and toolset macros, or gcc's and clang's -- predefines generated from the data model, the `__has_feature`/`__has_builtin`/`__has_attribute` operators, GNU builtins and `__asm` labels, and vcx's own compiler headers where a platform expects the compiler to supply them. The parser accepts both families' extensions everywhere; what the dialect decides is who vcx says it is.
