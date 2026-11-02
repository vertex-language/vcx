# Architecture

## Compilation

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

## Compilation Phases

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

# Go API

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

## In-Memory Compilation

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

# Repository Structure

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
├── offload/            The containers a device image travels in: NVIDIA's fat binary, the clang offload bundle
├── runtime/            The runtimes an offload program links: cudart over the driver, HIP glue over libamdhip64
│
├── include/            Compiler-provided headers
│   ├── gnu/            GNU-compatible stddef.h, stdint.h, limits.h, float.h, etc.
│   ├── cuda/           The CUDA wrapper read before every .cu, and a minimal cuda_runtime.h
│   ├── hip/            The same for HIP
│   └── metal/          The wrapper read before every .metal, and vcx's own <metal_stdlib>
│
├── compiler.go         Central Compiler driver orchestrating pipeline rungs
├── build.go            Build and Run: objects, the link, the temporary executable
├── link.go             The link, through the vertex-language ELF, Mach-O and PE linkers
├── device.go           Offload languages, architectures, predefines, the embedded headers
├── input.go            Input abstraction (file on disk or in-memory text buffer)
├── target.go           Target triples, ABI selection, and container mapping
├── codegen.go          Native object generation (ELF, Mach-O, PE/COFF), PTX, HSA code objects and Metal libraries
├── metal.go            A build of .metal files: one .metallib, or one per file with -c
├── symbols.go          Mangled linker symbol table extraction
├── sysroot.go          Host toolset discovery (MSVC, Windows SDK, GCC/Clang paths)
├── predefines.go       MSVC-dialect builtin macro definitions
├── predefines_gnu.go   GNU/Clang-dialect builtin macro definitions
├── vendor.go           Compiler intrinsics, builtins, and vendor extensions
│
├── tests/              The ladders: C++, CUDA, HIP, Metal and Objective-C++
└── docs/               Grammar and architectural specifications
```
