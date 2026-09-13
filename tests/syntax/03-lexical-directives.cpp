// §15 [cpp] -- phase 4 is a grammar of its own, and what survives it is what
// the parser sees. Nothing here has to mean anything: the file is about the
// directives, not the program they leave behind.

#define OBJECT 1
#define FUNCTION(a, b) ((a) + (b))
#define VARIADIC(fmt, ...) sink(fmt, __VA_ARGS__)
#define OPTIONAL(fmt, ...) sink(fmt __VA_OPT__(,) __VA_ARGS__)
#define STRINGIZE(x) #x
#define PASTE(a, b) a##b
#define EMPTY

#undef EMPTY

#if defined(OBJECT) && OBJECT == 1
int taken;
#elif defined(NOTHING)
int not_taken;
#else
int also_not_taken;
#endif

#ifdef OBJECT
int by_ifdef;
#endif

#ifndef NOTHING
int by_ifndef;
#endif

// §15.2 [cpp.include] -- both forms, and the one that is macro-expanded first.
#define HEADER <stddef.h>

#line 100 "renamed.cpp"
int after_line_directive;
#line 200

// §15.4 [cpp.pragma] -- an unrecognized pragma is ignored rather than
// refused. Phase 4 passes it through as `# pragma ...` on purpose, so that a
// pragma this compiler learns to honour later still has its operands, and
// phase 7 drops the line.
#pragma unknown_to_this_compiler
#pragma vendor_thing(with, operands, 1)

// §15.10 [cpp.pragma.op] -- the operator form, which is not a directive and
// so may appear wherever a token may. Its operand is destringized and read
// as though it had been written after a `#pragma`.
_Pragma("also ignored")
_Pragma("vendor_thing(1)")

// §15.11 [cpp.predefined] -- the ones that must exist.
int line = __LINE__;
const char* file = __FILE__;
long standard = __cplusplus;
int counter = __COUNTER__;

// The expansions themselves, so that the parser sees their result.
int obj = OBJECT;
int fn = FUNCTION(1, 2);
const char* str = STRINGIZE(text);
int PASTE(pas, ted) = 0;
