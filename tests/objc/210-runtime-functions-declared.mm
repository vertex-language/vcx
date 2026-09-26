// The program declares and calls runtime functions objv's own lowering also
// calls -- objc_msgSend through casts, the autorelease pool entry points,
// objc_retain and objc_release -- in the same unit as the constructs that
// make the compiler call them. Under ARC the program may not send retain or
// release itself, but may still call the functions they compile to.
#import <Foundation/Foundation.h>
#import <objc/message.h>
#include <stdio.h>

extern "C" {
void *objc_autoreleasePoolPush(void);
void objc_autoreleasePoolPop(void *);
id objc_retain(id);
void objc_release(id);
}

@interface Calc : NSObject
- (double)scale:(double)v by:(int)k;
- (int)count;
- (id)me;
@end
@implementation Calc
- (double)scale:(double)v by:(int)k { return v * k; }
- (int)count { return 3; }
- (id)me { return self; }
- (void)dealloc { printf("dealloc\n"); }
@end

int main(void) {
    Calc *c = [Calc new];
    double d = ((double (*)(id, SEL, double, int))objc_msgSend)(c, @selector(scale:by:), 1.5, 4);
    int n = ((int (*)(id, SEL))objc_msgSend)(c, @selector(count));
    id me = ((id (*)(id, SEL))objc_msgSend)(c, @selector(me));
    printf("%g %d %d\n", d, n, me == c);
    void *pool = objc_autoreleasePoolPush();
    objc_autoreleasePoolPop(pool);
    @autoreleasepool {
        (void)[c me];
    }
    objc_retain(c);
    objc_release(c);
    printf("end\n");
    return 0;
}
