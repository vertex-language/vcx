# tests

Eight corpora, asking eight different questions. Each is named for its
question, and a file belongs in exactly one of them.

```
go test ./...                                    # all of it
go test ./parser -run TestSyntaxCorpus -v
go test ./sema   -run 'TestCheckCorpus/ok-11' -v
go test ./sema   -run TestEvalCorpus -v
go test .        -run TestABICorpus -v
go test .        -run TestMangleCorpus -v
go test .        -run TestCompilerCorpus -v
go test .        -run TestLinkCorpus -v
go test .        -run TestHeadersCorpus -v
```

## One compiler, every target

vcx is one C++ compiler with targets, not a Windows compiler with other
targets added, and the corpora are written that way. A file directly in
`tests/<corpus>/` is portable C++ and every target runs it. What differs
between targets is the *answer*, and the answer always comes from the
platform's own compiler, never from the file: `cl` and `link.exe` for an
MSVC-dialect target, `clang++` and its linker for a GNU-dialect one --
Apple's on a Mac. A program whose result depends on the target, like one
that tests `sizeof(long)`, is still portable: both compilers are asked, and
they have to agree with each other on each machine.

A file that means something to one family of toolchains only lives one
level down, in `tests/<corpus>/msvc/` or `tests/<corpus>/gnu/`, and runs
only for a target of that dialect: `__declspec` in the first, `__asm`
labels and `__builtin_fabs` in the second. The folder is the whole
mechanism. Nothing in a file says which targets it is for, and no runner
keeps a list; `corpus_test.go` reads the folders. A result reported as
`gnu/084-builtins-and-asm-labels` is one of those.

The corpora that need an oracle skip, naming what is missing, on a machine
that has neither toolchain. The rest -- syntax, check, eval -- analyze
without a target and run everywhere.

## syntax/

Does it parse? 17 files covering the C++23 grammar, one per chapter of it,
from the lexical clause through the combinations that only break a parser
when they meet. Nothing here has to mean anything or run -- several files are
deliberately nonsense that happens to be well-formed -- so the only question
asked of them is whether the parser accepts what the grammar allows.

Every construct cites the paragraph it comes from, because there is no
oracle. The obvious candidate is `cl.exe`, and for phase 4 it is an excellent
one: it agrees token for token on the `[cpp.rescan]` examples, on
`__VA_OPT__`, on the placemarker in `a ## __VA_OPT__(x)`. For C++23 core
syntax it is not. It still rejects P2223's whitespace-before-newline splice
that GCC and Clang have warned about rather than refused for years; it has no
delimited escapes; it takes `0x1e+2` for a single pp-number where the grammar
makes that ill-formed. So the standard is the authority here and the
paragraph number is the citation.

Used by `parser`, in `corpus_test.go`.

## check/

Does it typecheck, and does it say something when it does not? Files named
`ok-*` must check clean -- not one diagnostic, because a warning on correct
code is a bug in the same way an error is. The rest must be rejected, and
each `bad-*` file is one violation with the paragraph it breaks written
above it.

What a rejection *says* is not asserted, and that is the point of writing the
paragraph down. A file of expected messages would be maintained by hand and
wrong the moment a diagnostic improved, which turns a better diagnostic into
a failing test -- exactly backwards. The file is the record instead:

```
v++ check tests/check/bad-07-private-member.cpp
```

prints the diagnostic, and reading that against the paragraph quoted in the
file is how the wording is reviewed. What the suite checks is the part a
machine can: that a violation is caught at all, and that correct code stays
quiet.

Used by `sema`, in `corpus_test.go`.

## eval/

Does the constant evaluator get the right answer?

This is the corpus that runs. Every file is a whole program whose assertions
are `static_assert`s, so compiling the file *is* running it: the loops
iterate, the arrays are written through, the recursion recurses, and a wrong
answer is a diagnostic rather than a number nobody reads. A compiler with no
code generator can still be asked to execute C++, and this is where it is
asked -- `tests/eval/06` sieves primes, sorts an array in place, walks a
Collatz path and computes an integer square root, all of it before a single
instruction exists to emit.

Nothing here writes an expected value into a harness. A table of numbers
beside a program is a claim maintained by hand and wrong the moment it
drifts; the claim belongs in the program, as `static_assert(fib(20) ==
6765)`, where it is checked by the thing it is a claim about. That is the
same reason `tests/compiler` gives in the sibling projects, arriving a phase
earlier.

`11` is the one that decides requires-expressions: every concept in it
has a type it rejects, because a constraint satisfied by everything is
indistinguishable from none, and the one wrong answer no diagnostic ever
catches is *true*.

It is also the one corpus with a working oracle. A `static_assert` file is
portable C++23 -- no headers, no entry point, no platform -- so

```
cl /nologo /std:c++latest /Zs tests\eval\02-constexpr-functions.cpp
```

compiles the same file unchanged and has to accept it too (`/utf-8`, as
the runners pass it: cl's default reads the source in the system code
page, and C++23 makes UTF-8 the portable case). There is no output
to compare, which is the elegant part: both compilers either accept the file
or name the assertion that failed, and agreeing on which is the whole
question. It is not wired into the runner, because this machine needs a
wrapper to give `cl.exe` an `%INCLUDE%` at all, but nothing in these files
stands in the way of running it.

One thing they do assert about the target: `04` checks `sizeof` and
`alignof`, which are LP64's. The runner analyzes under that model for that
reason, and a file added here must not assume the host's.

Used by `sema`, in `eval_test.go`.

## abi/

Does vcx lay a class out the way the platform's compiler does?

This is the corpus that decides whether the project works, and the only one
whose files state nothing at all. A layout test does not run and has no
expected values in it: the file is a set of class declarations, and the
numbers come from the other compiler. vcx computes a layout, `cl` is asked
for its own with `/d1reportSingleClassLayout`, and the two are compared:
size, base offsets, the offset of every named member including the inherited
ones, and every virtual table entry for entry -- which function a call
through slot *n* actually reaches.

That is not a stylistic preference. Every Windows layout question settled in
vcc came out differently from what the documentation implied, and each was
settled in minutes by writing the program twice and diffing. Four such
answers came out of writing these four files, none of which could have been
read off a specification:

- a **member pointer** under Microsoft is sized from the class's
  inheritance, not from the machine's pointer: four bytes for a data member
  even under multiple inheritance, eight under virtual inheritance, and
  eight or sixteen for a member function. Itanium gives every one of them
  the same size.
- **two empty bases cannot share an address** (§6.7.2/2), so the second one
  takes a byte -- but a class whose only base is empty is itself empty, and
  the test for that has to recurse.
- a **bit-field starts a new storage unit when the declared type changes**,
  so `unsigned char a : 3; unsigned int b : 3;` is eight bytes under `cl`
  and four under a compiler that packs across the change.
- a class with a **polymorphic base does not add a second vptr**, and the
  bases are **reordered** so the polymorphic one is first: `struct D :
  Plain, PolyA {}` lays PolyA out at zero, which is not the order it was
  written in.
- a **virtual base goes last**, after the non-virtual part is rounded up to
  the class's alignment -- and an **empty** one takes no bytes at all, not
  even the byte an empty complete object has, while still getting an offset
  one past the end that its table reports.
- the **vbptr is inherited like the vfptr**, so a class deriving from a
  diamond carries the two its bases brought and adds none; and a base's own
  table cannot serve, because its offsets are the ones that base computed
  for itself and the complete object put the shared subobject elsewhere.
- a **class-head `alignas` pads the complete object only**: a class
  derived from `struct alignas(16) A { int v; }` puts its first member
  at offset 4, inside what the base's `sizeof` counts, and rounds its
  own size up to 16 afterwards. An `alignas` on a member is the
  member's and does travel into a derived class.
- **overloads share consecutive vtable slots and are numbered backwards.**
  For `virtual void f(int); virtual void f(double); virtual void f(char);`
  cl gives f(char) slot 0 and f(int) slot 2, and a function declared
  *between* two overloads comes after both. Nothing states this; a derived
  class overriding one at a time is what pins it down.

Without `cl.exe` the corpus is skipped rather than checked against numbers
written down by hand -- those would be a record of what vcx did on the day
they were written, not of what is correct.

`v++ layout file.cpp` prints the same thing by hand, which is how a
disagreement is read once the runner reports one.

Used by the root package, in `abi_test.go`.

## mangle/

Does vcx name a definition the way the platform's compiler names it?

The other half of the ABI question. Two objects link only if the one that
defines `Widget::get() const` and the one that calls it spell it the same
way, and the spelling is a scheme nobody wrote down: Microsoft's is
documented by `cl`'s output and by nothing else. So the files are
declarations and the names come from `cl`. Each file is compiled with `/c`,
`dumpbin /symbols` lists what the object defines, and every name vcx would
give a definition in the file has to be in that list -- and every name `cl`
defined that came from the source has to be one vcx produced, so that a
function vcx could not name is a failure and not a silence.

The scheme has rules the files found that no description of it gives:

- **top-level const on a pointer parameter is kept**, `?cp@@YAXQEAH@Z` for
  `cp(int *const)`, though on anything else it is dropped as §9.3.4.6/5
  says; and a **decayed array parameter is a const pointer**, `QEAH` for
  `int[3]`.
- a **pointer to a const pointer writes the const twice**, as the pointee's
  cv letter and again as the inner pointer's own: `PEBQEBH`.
- a pointer **variable's trailing cv letter is the pointee's**, not its
  own: `?pc@@3PEBDEB` for `const char *pc`, where its own const would have
  been the `Q` in front.
- the **back-reference tables key on the whole type**, qualifiers at every
  level, so `void *` and `const void *` are two entries; and a two-letter
  basic type takes a slot where a one-letter one never does.
- a class with **several polymorphic bases names every table for its
  base**, the primary included -- `??_7Labelled@@6BShape@@@` and
  `??_7Labelled@@6BNamed@@@`, no plain `6B@` at all.
- an **override of a secondary base's virtual expects `this` to be that
  base's subobject.** There is no thunk in the table; the adjustment is
  folded into the callee, and a direct call adds the offset before
  calling. A thunk appears only where one function is reached through two
  subobjects at different offsets. This is the rule that changed lowering,
  and `tests/compiler/038` is the program that runs it.

`cl` emits an inline function only where it is used and synthesizes an
implicit constructor as a function where vcx does the work inline, so the
files define their members out of line and declare their constructors: the
corpus asks about names, and the compiler corpus is where a difference in
what gets emitted would show.

The Itanium scheme has no oracle on this machine -- no `g++`, no
`clang++` -- and is not in this corpus. It is written from the ABI document
and tested in `mangle/mangle_test.go` on examples the document gives, which
is the weaker kind of evidence and is labelled as such there.

`v++ symbols file.cpp` prints the same thing by hand.

Used by the root package, in `mangle_test.go`.

## compiler/

Does the program do what it says?

tests/syntax asks whether a file parses; tests/check whether it means
anything. This asks the only question after those, and the one every other
project in this tree is built around. Each file is a whole program with `int
main()`, compiled twice -- once by vcx, once by `cl` -- run twice, and the
two exit statuses have to agree.

Nothing here writes down an expected value. A number beside a program is a
claim about C++ that has to be maintained by hand and is wrong the moment it
drifts; `cl`'s answer cannot drift, because it is the platform's answer. And
both compilers are given exactly the same text -- there is no rewrite
between them, which is what makes the comparison worth anything. vcx's half
goes through its own lowering to VIR and its own object writer; only the
linker is shared, because a linker is not what is being tested.

The files are numbered in the order they get harder, and the early ones are
one idea each -- a loop, a call, a comparison -- so that a failure names the
thing that broke rather than the last thing added.

A file belongs here once vcx can build it: a refusal fails the suite rather
than being skipped, so the corpus stays a live statement of what works.

Used by the root package, in `compiler_corpus_test.go`.

## link/

Can a program be built half by vcx and half by `cl`?

tests/abi says the layouts agree and tests/mangle says the names agree,
and neither is the claim an ABI makes. That claim is that an object one
compiler built links against an object the other built and the program
works: every argument where the callee looks for it, a class constructed
on one side read correctly on the other, a virtual call through a table
one compiler emitted reaching a function the other compiled.

So each directory holds one program in two files, `a.cpp` and `b.cpp`,
each declaring what the other defines. It is built three ways -- both by
`cl`, `a` by vcx and `b` by `cl`, `a` by `cl` and `b` by vcx -- and the
three exit statuses have to agree. The first is the oracle and the other
two are the ABI, one direction each.

This is the corpus that made inline functions and virtual tables COMDAT
(the linker sees two definitions otherwise, and says so), that made a
global with a constructor get constructed before `main` (a `Labelled`
handed across from the other side had no table pointer), and that
demonstrated the callee-adjusted `this` convention at run time rather
than only in a name.

`07` is the four-byte class. cl returns a plain aggregate of one, two,
four or eight bytes in RAX, and the backend stored all eight bytes of
RAX into storage it had made four bytes wide -- an iterator's `end()`
overwritten by its `begin()`. The neighbours are checked on purpose.

`06` is where a virtual destructor's slot turned out to hold neither
destructor but the **deleting destructor** `??_E` -- destroy, and free
too when a flag says so -- which cl defines as `??_G` in every unit that
emits the table and aliases weakly; and where a **constructor returns
`this`** under this convention: cl's `new T(args)` takes the object from
the constructor's RAX, and the linker had chosen vcx's COMDAT copy of an
inline constructor that handed back nothing. That one crashed only when
the freed address was reused, and only across the boundary.

Used by the root package, in `link_corpus_test.go`.

## headers/

Does a standard header compile, from the toolset this machine has?

Every other corpus is text these tests wrote. This is text somebody else
wrote -- the Microsoft STL and the Windows SDK on Windows, libc++ and the
macOS SDK on a Mac, as installed -- which is the text every real program
starts with, and thousands of lines of the language's hardest corners
written for one compiler. Each file includes one header and uses something
from it, and is built and run the way tests/compiler builds and runs, with
the platform's compiler as the oracle; a header that only checks clean is
not enough, since the point of `<cstdint>` is that `int32_t` is four bytes
in the object.

The sysroot is found the way `v++ env` shows it -- vcc's walk over
%INCLUDE%, vswhere and the Kits root on Windows, the SDK and its libc++ on
a Mac -- and the target's macros are its dialect's. For MSVC they are the
ones `cl /PD` lists; for GNU they are clang's, generated from the data
model and held to `clang -dM` by `TestGNUPredefines`, and the vendor
operators -- `__has_feature`, `__has_builtin`, `__has_attribute` -- answer
with what vcx implements, which is what lets libc++ choose the paths vcx
can compile. What that costs is in `predefines.go`, `predefines_gnu.go`
and `vendor.go`; the compiler's own headers a GNU target needs, the ones
no platform ships, are in `include/gnu`.

Seven headers are in it. `<type_traits>` is the library's exercise of
variable templates, default template arguments, packs, fold expressions
and the choice among partial specializations; `<new>` adds placement new
and the runtime's exception classes (declared, since nothing throws yet);
`<utility>` adds member function templates, forwarding references and
the traits that are overload resolution in disguise. `<vector>` is next,
and what it waits on is exceptions and `new[]`.

Used by the root package, in `headers_test.go`.

## What is not here, and why

**Everything lowering does not reach yet.** There is no exception in
tests/compiler, no destructor of a global at exit, no member template
and no variable template, because lowering handles none of them. A
lambda cannot capture `this`; a pointer to member of a class with
multiple or virtual bases, or to a virtual function, is refused; a
consteval constructor of a class with members is not evaluated. The comparison rewrites of §12.2.2.3 are there, `!=` from `==` and
the four relationals from `<=>`, reversed forms included. The corpus grows as
lowering does, one idea per file, rather than being written ahead of it.

**The linker.** `v++ build` produces an object and stops; the corpus hands
that object to the platform's linker. A linker of vcx's own is a separate repository
(`pe`, `elf`, `macho`) and composing it is the rung above this one.

## The rule for adding one

Write the file, run it, and put it in the corpus that matches the question it
asks. Cite the paragraph rather than describing the behaviour -- the file
outlives the session that prompted it, and a paragraph number is checkable
where a summary is not.

If the compiler refuses something the file should contain, **say so in the
file**, naming the production and what happens instead:

```cpp
// §8.6.5 [stmt.ranged] is not here. `for (int v : c)` does not parse: the
// parser takes `int v : c` for a bit-field and then wants a second colon.
// It is the loop C++ code is actually written with, so it is the first
// thing this file gets back.
```

An omission that names itself is a bug report with a home. A file quietly
trimmed until it passes is a suite that agrees with the compiler about what
C++ is.
