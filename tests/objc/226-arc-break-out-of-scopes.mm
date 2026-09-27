// ARC: break and continue leave only the scopes they jump out of. A strong
// local declared outside a switch or loop is released once, when its own
// scope ends, however many cases break; one declared inside a case, a loop
// body or a block in them is released every time control leaves it. The
// shape is ui/window's: a +0 object from a global table held in a strong
// local, then a switch whose cases break -- vcx released the local on the
// break and again at the end of the function.
// mode: arc
#import <Foundation/Foundation.h>
#include <stdio.h>

static int live;

@interface Obj : NSObject
@property (nonatomic) int n;
@end
@implementation Obj
- (instancetype)init { if ((self = [super init])) live++; return self; }
- (void)dealloc { live--; }
@end

static NSMutableDictionary *table;

static Obj *lookup(int key) { return table[@(key)]; }

static int viaSwitch(int key, int which) {
    Obj *o = lookup(key);
    if (o == nil)
        return -1;
    int r = 0;
    switch (which) {
    case 1: r = o.n + 1; break;
    case 2: r = o.n + 2; break;
    default: r = o.n; break;
    }
    return r;
}

static int viaSwitchInnerLocal(int key, int which) {
    Obj *o = lookup(key);
    switch (which) {
    case 1: {
        Obj *inner = [Obj new];
        inner.n = o.n * 2;
        if (inner.n > 0)
            break;
        return 0;
    }
    case 2: {
        Obj *inner = lookup(key);
        (void)inner;
        break;
    }
    }
    return o.n;
}

static int viaLoop(int key) {
    Obj *o = lookup(key);
    int sum = 0;
    for (int i = 0; i < 6; i++) {
        Obj *step = [Obj new];
        step.n = i;
        if (i == 1)
            continue;
        if (i == 4)
            break;
        sum += step.n + o.n;
    }
    return sum;
}

static int viaSwitchInLoop(int key) {
    Obj *o = lookup(key);
    int sum = 0;
    for (int i = 0; i < 4; i++) {
        Obj *held = lookup(key);
        switch (i) {
        case 0: sum += held.n; break;
        case 1: continue;
        case 2: sum += o.n; break;
        default: break;
        }
        sum += 100;
    }
    return sum;
}

static int viaWhile(int key) {
    Obj *o = lookup(key);
    int n = 0;
    while (true) {
        Obj *t = [Obj new];
        if (++n == 3)
            break;
        (void)t;
    }
    do {
        Obj *u = lookup(key);
        if (u != nil)
            break;
    } while (false);
    return n + o.n;
}

int main(void) {
    __weak Obj *watch = nil;
    @autoreleasepool {
        table = [NSMutableDictionary dictionary];
        Obj *o = [Obj new];
        o.n = 10;
        table[@7] = o;
        watch = o;
        o = nil;
    }
    int total = 0;
    for (int round = 0; round < 50; round++) {
        @autoreleasepool {
            total += viaSwitch(7, round % 3);
            total += viaSwitchInnerLocal(7, round % 3);
            total += viaLoop(7);
            total += viaSwitchInLoop(7);
            total += viaWhile(7);
        }
    }
    printf("total %d\n", total);
    @autoreleasepool {
        printf("alive %d live %d n %d\n", watch != nil, live, lookup(7).n);
    }
    @autoreleasepool {
        [table removeObjectForKey:@7];
    }
    printf("after remove: alive %d live %d\n", watch != nil, live);
    return 0;
}
