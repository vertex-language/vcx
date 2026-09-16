// An extern "C" function declared inside a namespace and defined outside
// it is one function.
//
// §9.11 [dcl.link]/6 -- a name with C language linkage denotes the same
// entity whatever namespace it is declared in, so a declaration in one
// place and the definition in another are not two functions and are not
// two symbols. The declaration used to become an import of a name this
// same unit also defined, which the IR rejected as a duplicate.
//
// The runtime is where this came from: a suspending primitive declared
// inside the runtime's own namespace, so that code above it can take its
// address, and defined below in the extern "C" block with everything
// else the compiler calls.

namespace inner {
extern "C" int answer(int n);
extern "C" void note(int n);

// Its address, taken before the definition is anywhere in sight.
int (*const held)(int) = &answer;

int useHeld(int n) { return held(n); }
} // namespace inner

extern "C" {
int answer(int n) { return n * 3; }
void note(int n) { (void)n; }
}

// Declared in the namespace, called from outside it: one entity.
int callDirect(int n) { return answer(n); }

int main() {
    inner::note(1);
    return inner::useHeld(5) + callDirect(4) - 20;   // 15 + 12 - 20 = 7
}
