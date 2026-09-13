/* <stdarg.h> -- the compiler's own header, for a GNU-dialect target.
 *
 * No platform ships this one: a va_list is whatever the compiler's calling
 * convention makes it, so gcc and clang each carry their own, and so does
 * vcx. The names are the GNU builtins, which the analysis answers.
 */
#ifndef __VCX_STDARG_H
#define __VCX_STDARG_H

typedef __builtin_va_list va_list;
typedef __builtin_va_list __gnuc_va_list;

#define va_start(ap, param) __builtin_va_start(ap, param)
#define va_end(ap)          __builtin_va_end(ap)
#define va_arg(ap, type)    __builtin_va_arg(ap, type)
#define va_copy(dest, src)  __builtin_va_copy(dest, src)
#define __va_copy(d, s)     __builtin_va_copy(d, s)

#endif
