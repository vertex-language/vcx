# v++ (vcx)

<p align="center">
  <img src="https://img.shields.io/badge/language-C%2B%2B23-2563EB?style=flat-square&labelColor=0D1030" alt="C++23">
  <img src="https://img.shields.io/badge/compiler-0.0.0--dev-4F46E5?style=flat-square&labelColor=0D1030" alt="0.0.0-dev">
  <img src="https://img.shields.io/badge/go-1.23%2B-7C3AED?style=flat-square&labelColor=0D1030" alt="Go 1.23+">
  <img src="https://img.shields.io/badge/license-MIT-9333EA?style=flat-square&labelColor=0D1030" alt="MIT License">
</p>

**vcx** is the Vertex C++ compiler. It provides a from-scratch ISO C++23 front end written in Go, lowering directly to VIR (Vertex Intermediate Representation) and native relocatable object files (`.o` / `.obj`).

The CLI binary is `v++`; the Go module and repository is `github.com/vertex-language/vcx`.

```console
$ v++ build -o app.o main.cpp
$ v++ build --emit vir main.cpp
$ v++ check src/
$ v++ layout record.cpp
$ v++ symbols main.cpp
$ v++ ast main.cpp
$ v++ tokens main.cpp
$ v++ env
```

---

## Highlights

- **From-Scratch C++23 Front End**: Fully independent lexer, ISO phase 4 preprocessor, recursive-descent parser, semantic analyzer, and compile-time constant evaluator.
- **Dual ABI Parity**: Complete implementation of both the **Itanium C++ ABI** (Linux, macOS, bare-metal ELF) and the **Microsoft C++ ABI** (Windows MSVC layout, virtual base pointers/tables, name mangling).
- **Direct Lowering to VIR**: Monomorphized templates, virtual tables, cleanup scopes, and control flow lower directly into typed SSA VIR modules without intermediate external layers.
- **Native Relocatable Object Output**: Emits ELF, Mach-O, and PE/COFF relocatable object files directly via native Go encoders.
- **Self-Contained Pipeline**: Zero reliance on host `clang`, `gcc`, `as`, or LLVM.
- **Rich Introspection CLI**: Built-in commands to inspect token streams, AST nodes, computed record memory layouts, and mangled symbol tables.
- **Modular Go Library**: Every compilation stage is exposed as a composable, reusable Go API.

---

## Quick Start

### Installation

Compile and install `v++` using the Go toolchain:

```console
$ go install github.com/vertex-language/vcx/cmd/v++@latest
```

Or build directly from source:

```console
$ git clone https://github.com/vertex-language/vcx.git
$ cd vcx
$ go build -o v++ ./cmd/v++
```

### Basic Commands

```console
# Type check a translation unit without code generation
$ v++ check main.cpp

# Compile to a native relocatable object file
$ v++ build -o main.o main.cpp

# Inspect the lowered Vertex Intermediate Representation (VIR)
$ v++ build --emit vir main.cpp

# Inspect computed class memory layout (offsets, sizes, vtables, vbtables)
$ v++ layout main.cpp

# Inspect the mangled symbol table
$ v++ symbols main.cpp

# Dump the syntax tree
$ v++ ast main.cpp

# Dump the tokenized stream
$ v++ tokens main.cpp

# View resolved environment, target info, and system include directories
$ v++ env
```

---

## CLI Reference

`v++` uses a verb-first command structure:

```
v++ <command> [flags] [files...]
```

### Commands

| Command | Description |
| --- | --- |
| `v++ build` | Compile inputs to an object file (`.o` / `.obj`). With `--emit vir`, outputs VIR text. |
| `v++ run` | Compile and execute a source file. |
| `v++ check` | Run preprocessing, parsing, and semantic analysis; emit diagnostics without generating code. |
| `v++ layout` | Print the exact memory layout of every class/struct (sizes, base offsets, field offsets, vtables, vbtables). |
| `v++ symbols` | Print the object-file mangled symbol names for all definitions in the file. |
| `v++ ast` | Parse source into an AST and dump the tree representation. |
| `v++ tokens` | Run tokenization and print the token stream with source positions. |
| `v++ env` | Print resolved target architecture, default language standard, and discovered system include paths. |

### Common Flags

- `-target <triple>`: Compilation target (e.g. `x86_64-windows`, `x86_64-linux`, `aarch64-macos`). Defaults to the host system.
- `-std <version>`: C++ language standard: `c++20`, `c++23` (default), or `c++26`.
- `-I <dir>`: Add an include search directory (repeatable, searched in order).
- `-D <name>[=<val>]`: Define a preprocessor macro.
- `-U <name>`: Undefine a preprocessor macro.
- `-freestanding`: Compile for a freestanding environment (disables hosted standard library discovery).
- `-o <path>`: Output artifact path (used with `v++ build`).
- `--emit vir|obj`: Target output representation (`obj` produces relocatable object files; `vir` dumps IR).

### AST & Parsing Flags

- `-skip-bodies`: Skip parsing function bodies for faster structural passes.
- `-comments`: Retain source comments on AST nodes.

---

## Compiler Architecture

Translation follows the standard ISO C++ phase model implemented across clean, isolated Go packages:

```
                     Source Text
                          │
                   [ token / scanner ]
                    Phases 1, 2, 3
                 (Tokens, Sites, UCNs)
                          │
                   [ preprocessor ]
                       Phase 4
            (Macros, Directives, #include)
                          │
                      [ literal ]
                     Phases 5, 6
             (Escape Sequences, Concat)
                          │
                     [ parser ]
                       Phase 7
                  (Syntax Tree AST)
                          │
                 [ sema / constexpr ]
                     Phases 7, 8
         (Lookup, Overloads, Templates, CFG)
                          │
                   [ lower / mangle ]
                       Phase 8
        (VIR Builder, ABI Layout, Mangling)
                          │
                     VIR Module
                          │
             [ Native Object Generation ]
                       Phase 9
                  (ELF / Mach-O / PE)
```

### Compilation Phases

1. **Tokens & Lexing (`token`, `scanner`)**:
   Character mapping, universal character names (UCNs), raw string literals, trigraph handling, and position indexing (`token.Site`, `token.File`). Diagnostics are arena-allocated values tied to exact source positions.
2. **Preprocessing (`preprocessor`)**:
   Complete ISO Phase 4 preprocessor. Macro definition, expansion, stringification (`#`), token concatenation (`##`), `__VA_OPT__`, conditional inclusion (`#if`, `#ifdef`), and include resolution backed by abstracted `io/fs` filesystem mounts. Embeds standard freestanding compiler headers (`include/gnu/`) and discovers host toolsets (`sysroot.go`).
3. **Literal Decoding (`literal`)**:
   ISO Phases 5 and 6 string literal decoding, escape sequence translation, and literal concatenation across character encodings (narrow, UTF-8, UTF-16, UTF-32, wide).
4. **Parsing (`parser`, `ast`)**:
   Recursive-descent C++23 parser producing an explicit AST. Resilient parsing guarantees an `ast.File` is produced even in the presence of syntax errors for robust diagnostics and IDE tooling.
5. **Semantic Analysis (`sema`, `types`, `constexpr`)**:
   Type checking, unqualified and qualified name lookup, argument-dependent lookup (ADL), two-phase template lookup, overload resolution, template argument deduction and instantiation, implicit conversions, and special member synthesis.
   - `sema/cfg`: In-memory control-flow graphs for definite return checks, unreachable code analysis, and destructor sequencing.
   - `constexpr`: Integrated virtual machine evaluator capable of executing loops, recursion, and object mutations at compile time for `static_assert` and template metaprogramming.
6. **Lowering & ABI (`lower`, `mangle`, `types`)**:
   Lowers the elaborated AST into Vertex Intermediate Representation (VIR). Handles class layout, vtable/vbtable construction, cleanup scope emission for unwinding, constructor/destructor sequencing, and symbol mangling under both the Itanium and Microsoft ABIs.
7. **Code Generation (`codegen.go`)**:
   Generates native relocatable object files (ELF, Mach-O, PE/COFF) via native Vertex architecture encoders.

---

## Targets and ABIs

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

Target selection handles:
- **Data models**: LP64 (Linux/macOS), LLP64 (Windows), ILP32 (i386), with Apple arm64's 64-bit `long double`.
- **C++ ABI** (`types.CXXABI`): Microsoft layout, vftables and vbtables; Itanium layout with tail-padding reuse and vtable groups, and its ARM and Apple arm64 variants -- structors that return `this`, 16-byte array cookies, C++11 POD rules for tail padding.
- **Name mangling**: Itanium (`_Z...`) or Microsoft (`?...`).
- **Dialect**: cl's identity and toolset macros, or gcc's and clang's -- predefines generated from the data model, the `__has_feature`/`__has_builtin`/`__has_attribute` operators, GNU builtins and `__asm` labels, and vcx's own compiler headers where a platform expects the compiler to supply them. The parser accepts both families' extensions everywhere; what the dialect decides is who vcx says it is.

---

## Go API

`vcx` is organized so that every compiler phase can be used as a standalone Go library:

```go
package main

import (
	"fmt"
	"log"

	"github.com/vertex-language/vcx"
)

func main() {
	// Simple one-line build
	if err := vcx.Build("app.o", "main.cpp"); err != nil {
		log.Fatalf("build failed: %v", err)
	}

	// Advanced configuration with Compiler driver
	c := &vcx.Compiler{
		Target:      "x86_64-windows",
		Std:         vcx.Cxx23,
		IncludeDirs: []string{"include"},
	}

	// Check for diagnostics
	diags, err := c.Check(vcx.File("main.cpp"))
	if err != nil {
		log.Fatal(err)
	}
	for _, d := range diags {
		fmt.Printf("%s: %s: %s\n", d.Site, d.Severity, d.Message)
	}

	// Lower directly to VIR
	mod, diags, err := c.IR(vcx.File("main.cpp"))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Emitted VIR module: %s (%d functions)\n", mod.Name(), len(mod.Funcs()))
}
```

### In-Memory Compilation

Source inputs do not need to exist on disk. `vcx.Text` allows compiling code directly from memory:

```go
src := []byte(`
struct Point {
    int x;
    int y;
};
`)

c := &vcx.Compiler{}
records, _, err := c.Layout(vcx.Text("point.cpp", src))
if err != nil {
	log.Fatal(err)
}

for _, r := range records {
	fmt.Printf("%s %s (size=%d, align=%d)\n", r.Kind, r.Name, r.Size, r.Align)
}
```

---

## Repository Structure

```
vcx/
├── cmd/v++/            Binary entry point for the v++ CLI tool
├── internal/cli/       CLI command routing, flag handling, and formatting
│
├── token/              Phases 1–2: tokens, source sites, files, position tracking
├── scanner/            Phase 3: tokenization, literals, comments
├── preprocessor/       Phase 4: directives, macro expansions, include resolution
├── literal/            Phases 5–6: string literal decoding, escape sequences, concatenation
├── parser/             Phase 7: recursive descent C++ parser, error recovery
├── ast/                AST node definitions and tree inspection utilities
│
├── sema/               Semantic analysis: lookup, overload resolution, templates
│   └── cfg/            Control flow graph builder for flow diagnostics and cleanups
├── types/              Type representations, size/alignment models, ABI layout
├── constexpr/          Compile-time constant expression virtual machine
├── lower/              Lowering from elaborated AST to Vertex Intermediate Representation
├── mangle/             Symbol manglers for Itanium and Microsoft ABIs
│
├── include/            Compiler-provided freestanding standard headers
│   └── gnu/            GNU-compatible stddef.h, stdint.h, limits.h, float.h, etc.
│
├── compiler.go         Central Compiler driver orchestrating pipeline rungs
├── input.go            Input abstraction (file on disk or in-memory text buffer)
├── target.go           Target triples, ABI selection, and container mapping
├── codegen.go          Native object generation (ELF, Mach-O, PE/COFF)
├── symbols.go          Mangled linker symbol table extraction
├── sysroot.go          Host toolset discovery (MSVC, Windows SDK, GCC/Clang paths)
├── predefines.go       MSVC-dialect builtin macro definitions
├── predefines_gnu.go   GNU/Clang-dialect builtin macro definitions
├── vendor.go           Compiler intrinsics, builtins, and vendor extensions
│
├── tests/              Comprehensive test suites (8 corpora)
└── docs/               Grammar and architectural specifications
```

---

## Testing & Conformance

`vcx` uses eight test corpora organized by the specific verification question they answer:

```console
$ go test ./...                                    # Run the full test suite
$ go test ./parser -run TestSyntaxCorpus -v        # Grammar syntax coverage
$ go test ./sema   -run 'TestCheckCorpus/ok-11' -v # Semantic type checking
$ go test ./sema   -run TestEvalCorpus -v          # Compile-time constexpr evaluation
$ go test .        -run TestABICorpus -v           # Record layout & vtable parity
$ go test .        -run TestMangleCorpus -v        # Name mangling parity
$ go test .        -run TestCompilerCorpus -v      # End-to-end execution parity
$ go test .        -run TestLinkCorpus -v          # Cross-compiler link tests
$ go test .        -run TestHeadersCorpus -v       # Real standard library headers
```

- **`syntax/`**: Validates parser conformance against the ISO C++23 grammar chapters.
- **`check/`**: Verifies that well-formed code (`ok-*`) type-checks cleanly and ill-formed code (`bad-*`) produces diagnostics citing the relevant ISO paragraph.
- **`eval/`**: Self-checking `static_assert` programs that execute complex algorithms (e.g. prime sieves, in-place sorting, Collatz paths) entirely within `constexpr`.
- **`abi/`**: Verifies class layouts, field offsets, bit-field allocations, empty base optimization, and virtual table structures against native compilers (`cl.exe`, `clang++`).
- **`mangle/`**: Verifies exact symbol mangling agreement for MSVC and Itanium schemes against object dumps.
- **`compiler/`**: End-to-end execution tests where programs are compiled and run to verify correct runtime behavior.
- **`link/`**: Verifies ABI compatibility by linking objects compiled by `v++` with objects compiled by the host toolchain.
- **`headers/`**: Tests compilation against real installed system headers, including `<type_traits>`, `<utility>`, `<new>`, and `<cstdint>`.

---

## Status

`v++` has a functional front end and lowering pipeline:

- **Lexer & Preprocessor**: Fully compliant ISO phase 1–4 handling, including macro varargs (`__VA_OPT__`), token concatenation, and system include resolution.
- **Literal Decoding**: ISO phase 5–6 decoding and concatenation for narrow, UTF-8, UTF-16, UTF-32, and wide strings.
- **Parser & AST**: Parses C++23 constructs including templates, concepts, `requires` clauses, lambdas, structured bindings, and attributes.
- **Semantics**: Complete scope tracking, unqualified/qualified lookup, ADL, overload resolution, template argument deduction, and type conversion graphs.
- **Constexpr Evaluation**: Full compile-time VM running loops, conditionals, structs, arrays, and recursion.
- **ABI Engine**: Computes exact class layouts, bitfields, inheritance hierarchies, virtual bases, and virtual tables for both Itanium and Microsoft ABIs.
- **Lowering & Code Generation**: Lowers functions, locals, expressions, control flow, dynamic initialization, copy constructors, and polymorphic virtual dispatch to VIR and native object files.
- **System Header Support**: Directly compiles and links against host library headers such as `<type_traits>`, `<utility>`, and `<new>`.

---

## Documentation

- [C++ Grammar Specification](docs/cpp_grammar.md)
- [VIR Specification](https://github.com/vertex-language/ir)

---

## License

MIT License. See [LICENSE](LICENSE) for details.
