// Does an object with linkage get the name cl gives it?
//
// A variable's name carries its type and its cv, with an array spelled as
// a pointer to its element -- and without the `E` a pointer variable gets.
// A static data member says its access where a namespace-scope object says
// `3`.

int g;
const int c = 3;
volatile int vol;
int *p;
const char *pc;
char *const cp = nullptr;
int arr[3];
const int carr[2] = {1, 2};
double grid[2][3];
struct S { int v; static int count; static const int limit; };
int S::count;
const int S::limit = 4;
S obj;
const S cobj = {1};
S *pobj;
int (*fp)(int);
long long ll;
unsigned char uc;
