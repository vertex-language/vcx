/* <stdalign.h> -- the compiler's own header, for a GNU-dialect target.
 * In C++ alignas and alignof are keywords, and the header only says so.
 */
#ifndef __VCX_STDALIGN_H
#define __VCX_STDALIGN_H
#ifndef __cplusplus
#define alignas _Alignas
#define alignof _Alignof
#endif
#define __alignas_is_defined 1
#define __alignof_is_defined 1
#endif
