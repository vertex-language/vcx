// Is a namespace-scope aggregate initialized from its braced list?
//
// §6.9.3.2 [basic.start.static] -- an object with static storage duration
// whose initializer is a constant expression is constant-initialized: its
// image is in the object file before any code runs. §9.4.2 [dcl.init.aggr]
// -- the list fills members and elements in order, nested braces for a
// nested aggregate, and what the list leaves out is value-initialized.
// One whose initializer is not constant -- an address of another object,
// a call -- is initialized before main (§6.9.3.3), in declaration order.

struct Node { int v; int kids[2]; };
struct Named { const char* name; int len; };
struct Pt { short x, y; };
struct Line { Pt a, b; };

Node tree[3] = {{1, {1, 2}}, {2, {-1, -1}}, {3, {-1, -1}}};
int flat[6] = {1, 2, 3};                 // the rest zero
double reals[2] = {1.5, 2.25};
char text[8] = "abc";
Line diag = {{0, 0}, {3, 4}};
Pt pts[2] = {{1, 2}, {3, 4}};
const char* words[] = {"one", "two", "three"};   // addresses: before main
Named named[2] = {{"ab", 2}, {"cde", 3}};
int* into = &flat[2];
Node* first = &tree[0];
unsigned char bytes[4] = {0xFF, 1, 2};
bool flags[3] = {true, false, true};

int len(const char* s) { int n = 0; while (s[n]) ++n; return n; }
int sum(int i) { if (i < 0) return 0; return tree[i].v + sum(tree[i].kids[0]) + sum(tree[i].kids[1]); }

int main() {
    int r = 0;
    if (sum(0) == 6 && tree[2].kids[1] == -1) r += 1;
    if (flat[2] == 3 && flat[3] == 0 && flat[5] == 0) r += 1;
    if (reals[1] == 2.25 && reals[0] * 2 == 3.0) r += 1;
    if (text[2] == 'c' && text[3] == 0 && text[7] == 0) r += 1;
    if (diag.b.x == 3 && diag.b.y == 4 && diag.a.y == 0) r += 1;
    if (pts[1].x == 3 && pts[0].y == 2) r += 1;
    if (len(words[2]) == 5 && sizeof(words) / sizeof(words[0]) == 3) r += 1;
    if (named[1].len == 3 && len(named[0].name) == 2) r += 1;
    if (*into == 3 && first->kids[1] == 2) r += 1;
    if (bytes[0] == 255 && bytes[3] == 0 && flags[0] && !flags[1] && flags[2]) r += 1;
    return r; // ten checks
}
