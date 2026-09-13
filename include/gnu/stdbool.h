/* <stdbool.h> -- the compiler's own header, for a GNU-dialect target.
 * In C++ bool, true and false are keywords, and the header only says so.
 */
#ifndef __VCX_STDBOOL_H
#define __VCX_STDBOOL_H
#ifndef __cplusplus
#define bool  _Bool
#define true  1
#define false 0
#endif
#define __bool_true_false_are_defined 1
#endif
