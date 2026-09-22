# tests

A ladder: one small thing per file, numbered in the order the rungs climb.
The root is C++, `001`–`250`, and each offload language has its own ladder
of `001`–`050` beside it: `cuda/`, `hip/` and `metal/`.

Nothing here writes down an expected value. Every file is built twice,
once by vcx and once by the language's own compiler, and the two results
are compared. The other compiler's answer is the oracle, and a
disagreement with it is a bug in vcx by definition.

| Ladder | Files | The oracle | Compared |
| --- | --- | --- | --- |
| `tests/` | `001`–`250` `.cpp` | clang++ on macOS and Linux, cl on Windows | stdout and the exit status |
| `tests/cuda/` | `001`–`050` `.cu` | nvcc, on a machine with an NVIDIA GPU | stdout and the exit status |
| `tests/hip/` | `001`–`050` `.hip` | hipcc, on a machine with an AMD GPU and ROCm | stdout and the exit status |
| `tests/metal/` | `001`–`050` `.metal` | `xcrun metal`, on a Mac's GPU | every buffer after the dispatch |

Where no GPU or oracle is present, the CUDA and HIP programs are still
compiled through both passes, and must compile. The Metal ladder needs
the Metal toolchain (`xcodebuild -downloadComponent MetalToolchain`) and
skips without it.

## C++: `001`–`250`

| | |
| --- | --- |
| 001–020 | the smallest programs, then arithmetic one operator at a time: add, sub, mul, div, mod, shifts, bitwise, compares, logical, `?:`, increments, compound assignment |
| 021–040 | every scalar type: `char`, `short`, `long long`, `<cstdint>`, `bool`, promotion, the usual conversions, truncation, extension, `float`, `double`, NaN and infinity, literals, `sizeof` and `alignof` |
| 041–060 | control flow: `if`, the loops, `break` and `continue`, `switch` dense and sparse, `goto`, recursion, init-statements, the comma, statics, globals, dynamic initialization |
| 061–080 | pointers, arrays, C strings, `<cstring>`, references, `const`, `nullptr`, `void*`, `snprintf`, a sort |
| 081–100 | functions: overloading, default arguments, `inline`, function pointers, lambdas and captures, structs by value, layout, unions, bit-fields, C varargs |
| 101–120 | classes: constructors, destructors, copy and move, `this`, statics, `const` members, operators, `<=>`, friends, nested classes, `new` and `delete`, the rule of three, temporaries |
| 121–140 | inheritance: construction order, virtual functions, abstract classes, virtual destructors, `override` and `final`, multiple and virtual bases, access, hiding, slicing, covariance, `dynamic_cast`, `typeid`, empty bases |
| 141–160 | templates: function and class templates, non-type parameters, full and partial specialization, variadics, folds, template template parameters, `constexpr`, `if constexpr`, traits, SFINAE, concepts, `requires`, deduction, CTAD, variable and member templates |
| 161–180 | the rest of the language: `consteval`, `constinit`, structured bindings, range-for, enums, namespaces, aliases, `initializer_list`, user-defined literals, CRTP, raw strings, `alignas`, the casts, local classes; then exceptions, unwinding, hierarchies, `catch (...)`, `noexcept` |
| 181–200 | the standard library: `move` and `forward`, `pair` and `tuple`, `array`, `vector`, `string`, `<algorithm>`, `<numeric>`, `unique_ptr`, `shared_ptr`, `optional`, `variant`, `function`, `map`, `set`, `unordered_map`, `<cmath>`, iostreams, `string_view`, and a program that uses most of it |
| 201–210 | the object model: placement `new`, a class's own `operator new`, pointers to data and function members, `mutable`, `volatile`, `explicit(bool)`, anonymous unions, unions with non-trivial members, aggregates with bases |
| 211–220 | functions and lambdas, further: init-captures, `this` and `*this` captures, template lambdas, `constexpr` and immediately-invoked lambdas, recursion and overloads through deducing `this`, abbreviated templates, ADL, overload ranking |
| 221–230 | templates, further: dependent names, `auto` and class-type non-type parameters, `index_sequence` and `apply`, type erasure, tag dispatch, explicit instantiation and friend templates, concept subsumption, constrained members, metaprogramming, `inline` and `static constexpr` members |
| 231–240 | the rest of the language: `using enum` and bitmask enums, attributes, `if consteval`, leaving scopes by `break`, `continue`, `goto` and `return`, `thread_local`, static destruction order and `atexit`, function-try-blocks, `exception_ptr` and nested exceptions, `uncaught_exceptions`, `__int128`, `long double` and `<bit>` |
| 241–250 | more of the library: `std::expected`, ranges and views, `span`, the sequence containers and adaptors, `bitset`, a custom iterator under the algorithms, `std::format`, a coroutine generator, and a closing program over the later rungs |

## CUDA and HIP: `001`–`050`

Whole programs, each with a `main` that allocates, launches and prints.
The two ladders climb the same way, each in its own API:

| | |
| --- | --- |
| 001–010 | a kernel launched, a value written and read back, thread and block indices, vector add, bounds checks, grid-stride loops, 2D grids and 3D blocks, every scalar parameter type |
| 011–019 | structs by value, device functions, `__host__ __device__`, recursion, integer and 64-bit arithmetic, exact float functions, `double`, the math library, conversions |
| 020–027 | shared memory and barriers, reductions, dynamic shared memory, the atomics on integers and floats, `__constant__` and `__device__` globals through the symbol API |
| 028–033 | the warp or wavefront: shuffles, votes, bit intrinsics, `memset` and device-to-device copies, kernel sequences |
| 034–041 | streams, events, kernel templates, lambdas and classes on the device, device `printf`, `__launch_bounds__`, the vector types |
| 042–050 | tiled matrix multiply, a prefix scan, a bitonic sort, block-wide votes or scoped atomics, launch errors, device queries, managed and pinned memory, and a fixed-point Mandelbrot |

The HIP ladder is written against `warpSize`, since an AMD GPU's wavefront
is 64 lanes or 32; its ballots are 64 bits wide.

## Metal: `001`–`050`

A `.metal` file has no host code, so it says how to run it, on `//!`
lines:

```metal
//! kernel vector_add
//! grid 1000 64            threads, threads per threadgroup (or x y px py for 2D)
//! buffer float 1000 rand  [[buffer(0)]]: float, uint or int; its length; its contents
//! buffer float 1000 zero  [[buffer(1)]]
//! ulp 4                   float results may differ by this many ulps
```

A buffer's contents are `zero`, `iota` (its index), `rand` (a fixed
pseudo-random sequence: [0, 1) for a float, every bit for an integer),
`mod N` (`rand` modulo N) or `const V`.

| | |
| --- | --- |
| 001–018 | a constant stored, the grid position, vector add, then every integer and float operator, conversions, `?:`, and the control flow |
| 019–027 | device functions, templates, structs in buffers, `constant` references and pointers, binding order and implicit indices, program-scope tables, thread arrays, pointer arithmetic |
| 028–035 | threadgroup memory and barriers, a reduction, a 2D transpose, the atomics on device and threadgroup memory, integer and float |
| 036–042 | the SIMD group: shuffles, votes, reductions, prefix sums; the threadgroup and SIMD-group built-ins, partial threadgroups, 2D grids |
| 043–050 | exact and fast float functions, integer functions, `as_type`, namespaces and function objects, structs with operators, the vector types, and a fixed-point Mandelbrot |

## Rules

- **One thing per file.** A failure should name what broke. When a test
  turns out to be asking two questions, split it.
- **No undefined behavior, and nothing unspecified.** Signed overflow,
  out-of-range float-to-int, division by zero and `INT_MIN / -1` are left
  out, as is the order in which a function's arguments or an operator's
  operands are evaluated: both compilers are allowed to answer anything.
- **Floating point is exact where it can be.** Inputs are chosen so that a
  fused multiply-add and a separate multiply and add round the same, since
  nvcc fuses by default and vcx does not yet. Only the transcendentals are
  printed to fewer digits, or compared within stated ulps.
- **Each run is capped** at 10 seconds and 1 MB of output, so a
  miscompiled loop fails its own test instead of the whole run.
- **Deterministic output only**: no clock, no addresses, no hash order, no
  threads racing to print. An unordered container is sorted before it is
  printed; a device `printf` comes from one thread; an atomic float sum
  adds values whose total is exact in any order.
- **Every fix lands with the smallest numbered file that shows it.**

## Running

```console
$ go test -run TestCorpus .           # the C++ ladder
$ go test -run 'TestCorpus/113' .     # one rung
$ go test -run TestCUDA .             # the CUDA ladder
$ go test -run TestHIP .              # the HIP ladder
$ go test -run TestMetal .            # the Metal ladder
```
