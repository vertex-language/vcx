/* <stddef.h> -- the compiler's own header, for a GNU-dialect target.
 *
 * A hosted platform that has one of its own is deferred to: it knows its
 * max_align_t and its NULL. Freestanding, or on a platform without one,
 * the definitions come from the predefined macros, which are the same
 * types the compiler lays objects out with.
 */
#if __STDC_HOSTED__ && __has_include_next(<stddef.h>)
#include_next <stddef.h>
#else
#ifndef __VCX_STDDEF_H
#define __VCX_STDDEF_H

typedef __SIZE_TYPE__    size_t;
typedef __PTRDIFF_TYPE__ ptrdiff_t;
#ifndef __cplusplus
typedef __WCHAR_TYPE__   wchar_t;
#endif
typedef long double      max_align_t;

#undef NULL
#ifdef __cplusplus
#define NULL nullptr
#else
#define NULL ((void *)0)
#endif

#define offsetof(type, member) __builtin_offsetof(type, member)

#endif
#endif
