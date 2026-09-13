// §9.2 [dcl.spec] -- the decl-specifier-seq, which is a set rather than a
// sequence: the order the specifiers are written in does not matter, and
// this file writes several of them in the order nobody uses.

// §9.2.2 storage-class-specifiers.
static int s;
extern int e;
thread_local int t;
static thread_local int st;
mutable int not_here;   // only in a class; the specifier still parses

// §9.2.3 [dcl.fct.spec] and the constant-expression specifiers of §9.2.6.
inline int fi();
constexpr int fce() { return 0; }
consteval int fcv() { return 0; }
constinit int ci = 0;
static constexpr int scx = 1;

// §9.2.9 [dcl.type] -- the simple-type-specifiers, and every combination the
// grammar allows to name one of the fundamental types of §6.8.2.
signed char sc;
unsigned char uc;
short sh; short int shi; signed short ss; unsigned short us;
int i; signed si; unsigned ui; signed int sint; unsigned int uint;
long l; long int li; unsigned long ul; long unsigned int lui;
long long ll; unsigned long long ull; long long int lli;
float f; double d; long double ld;
bool b;
char c; char8_t c8; char16_t c16; char32_t c32; wchar_t wc;
void* pv;
decltype(nullptr) np;

// The order is free, which is why this is legal and unreadable.
int unsigned const volatile static reordered = 0;
const unsigned long int also = 0;
volatile int vol;
const volatile int cv = 0;

// §9.2.9.5 [dcl.type.auto.deduct] and §9.2.9.3 [dcl.type.decltype].
auto a = 1;
decltype(a) da = a;
decltype(auto) dda = a;
const auto ca = 1;
auto* pa = &a;
auto& ra = a;
auto&& rra = 1;

// §9.2.9.6 [dcl.type.elab] -- an elaborated-type-specifier names a type that
// may not have been declared yet.
struct Fwd;
class CFwd;
union UFwd;
enum class EFwd : int;
struct Fwd* pf;
enum class EFwd : int { A };

// §9.5 [dcl.typedef] and §9.2.4 [dcl.typedef] by its other spelling.
typedef int Integer;
typedef int (*FnPtr)(int, int);
typedef int Array[4];
using Alias = int;
using FnAlias = int (*)(int);
using ArrAlias = int[4];

// §9.6 [namespace.def] and §9.7 [namespace.udecl].
namespace N { int x; namespace Inner { int y; } }
namespace Alias2 = N::Inner;
namespace { int internal; }
inline namespace V1 { int versioned; }
using N::x;
using namespace N;
namespace N::Nested::Deep { int z; }   // §9.6.1/2, the nested definition

// §9.9 [dcl.asm] and §9.10 [dcl.link].
extern "C" int c_linkage(int);
extern "C++" int cxx_linkage(int);
extern "C" { int in_a_block(int); int and_another(int); }
