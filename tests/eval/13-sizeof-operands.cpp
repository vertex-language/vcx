// Does sizeof read the type of an expression operand, whatever it looks like?
//
// §7.6.2.5 [expr.sizeof] -- sizeof(unary-expression) is the size of the
// operand's type, the operand unevaluated; and §7.6.2.5/2 makes no
// array-to-pointer conversion, so an array operand is the whole array.
// The trap is the parser's: `sizeof(arr[0])` is not `sizeof(type-id)`
// with a type named arr, however much `arr[0]` looks like an array
// declarator (§8.9/2 resolves the same ambiguity for a statement).

struct Node { int v; int kids[2]; };

Node tree[7];
int ints[10];
const char* words[] = {"a", "bb", "ccc"};
Node* p = tree;
Node single;

static_assert(sizeof(tree) == 84);
static_assert(sizeof(tree[0]) == 12);
static_assert(sizeof(tree[1].kids) == 8);
static_assert(sizeof(tree[1].kids[1]) == 4);
static_assert(sizeof(ints) / sizeof(ints[0]) == 10);
static_assert(sizeof(words) / sizeof(words[0]) == 3);
static_assert(sizeof(*p) == 12);
static_assert(sizeof(p[2]) == 12);
static_assert(sizeof(p->v) == 4);
static_assert(sizeof(single.kids) == 8);
static_assert(sizeof(single) == sizeof(Node));
static_assert(sizeof(1 + 2) == sizeof(int));
static_assert(sizeof(1.0f * 2) == sizeof(float));
static_assert(sizeof('a') == 1);
static_assert(sizeof("ab") == 3);

constexpr int locals() {
    int arr[5] = {};
    Node n{};
    Node* q = &n;
    return (int)(sizeof(arr) / sizeof(arr[0])) + (int)sizeof(n.kids) + (int)sizeof(q->kids[0]) + (int)sizeof(*q);
}
static_assert(locals() == 5 + 8 + 4 + 12);
