// A C++ declaration written among a class's members belongs to the file:
// its name is in scope after @end. AppKit declares `extern NSApplication
// *NSApp` inside `@interface NSApplication`, and CoreImage `typedef
// NSString *CIImageOption` inside `@interface CIImage`.
// mode: both
#import <Foundation/Foundation.h>
#include <stdio.h>

@interface Shape : NSObject
typedef int ShapeKind;
extern Shape *theShape;
enum { ShapeSides = 4 };
- (ShapeKind)kind;
@end

@protocol Named
typedef const char *NameText;
- (NameText)name;
@end

@interface Shape () <Named>
@end

@implementation Shape
static int made;
- (ShapeKind)kind { return 7; }
- (NameText)name { return "square"; }
@end

Shape *theShape;

int main(void) {
    @autoreleasepool {
        theShape = [Shape new];
        made++;
        ShapeKind k = [theShape kind];
        NameText n = [theShape name];
        printf("%d %s %d %d\n", k, n, (int)ShapeSides, made);
    }
    return 0;
}
