# Command line

`v++` uses a verb-first command structure:

```
v++ <command> [flags] [files...]
```

## Commands

| Command | Description |
| --- | --- |
| `v++ build` | Compile inputs and link an executable. `-c` stops at objects (`.o` / `.obj`); `--emit vir` prints VIR text; `--emit ptx` / `--emit hsaco` the device image of a CUDA or HIP unit. |
| `v++ run` | Build a source file to a temporary executable and run it; arguments after the file are the program's. |
| `v++ check` | Run preprocessing, parsing, and semantic analysis; emit diagnostics without generating code. |
| `v++ layout` | Print the exact memory layout of every class/struct (sizes, base offsets, field offsets, vtables, vbtables). |
| `v++ symbols` | Print the object-file mangled symbol names for all definitions in the file. |
| `v++ ast` | Parse source into an AST and dump the tree representation. |
| `v++ tokens` | Run tokenization and print the token stream with source positions. |
| `v++ env` | Print resolved target architecture, default language standard, and discovered system include paths. |

## Common flags

- `-target <triple>`: Compilation target (e.g. `x86_64-windows`, `x86_64-linux`, `aarch64-macos`). Defaults to the host system.
- `-std <version>`: C++ language standard: `c++20`, `c++23` (default), or `c++26`.
- `-I <dir>`: Add an include search directory (repeatable, searched in order).
- `-D <name>[=<val>]`: Define a preprocessor macro.
- `-U <name>`: Undefine a preprocessor macro.
- `-freestanding`: Compile for a freestanding environment (disables hosted standard library discovery).
- `-o <path>`: Output artifact path (used with `v++ build`).
- `-c`: Compile to objects without linking; `-l <lib>` and `-L <dir>` name libraries for the link.
- `--emit vir|obj|ptx|hsaco|metallib|air|ll`: Target output representation (`obj` produces relocatable object files; `vir` dumps IR; `ptx` and `hsaco` the device image; for a `.metal` file, `air` its bitcode and `ll` its text).
- `-x cuda|hip|metal`: The language, whatever the extension says; `.cu` and `.cuh` are CUDA, `.hip` is HIP and `.metal` is Metal on their own.
- `--offload-arch <arch>`: The device kernels are compiled for: `sm_75`, `sm_90a`, `gfx942`, `apple8`, ... (default `sm_52` for CUDA and `apple7` for Metal; HIP has no default).
- `-std=metal3.0|metal3.1|metal3.2|metal4.0`, `-mmacosx-version-min=<v>`, `-fno-fast-math`: A `.metal` file's MSL version (default 3.0), the oldest macOS its library loads on (default 13.0, which is AIR 2.5), and precise float math (fast is Metal's default), as `xcrun metal` spells them.
- `--cuda-device-only`, `--cuda-host-only`: One pass of an offload unit rather than both.

## AST and parsing flags

- `-skip-bodies`: Skip parsing function bodies for faster structural passes.
- `-comments`: Retain source comments on AST nodes.
