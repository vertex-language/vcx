// Do pointers compare, subtract and convert the way §7.6 says?
//
// §7.6.10/1 [expr.eq] -- a pointer compares with a null pointer constant,
// which is a comparison and not the pointer arithmetic §7.6.6/4 gives
// `p + 0`; §7.6.9 -- two pointers into one array order by element;
// §7.6.6/5 -- their difference counts elements; §7.6.1.10/4-5 -- a pointer
// round-trips through an integer wide enough. §7.3.12 -- nullptr, 0 and
// a null pointer of another type are all null.

struct T { int v; };

int main() {
    int r = 0;

    int arr[5] = {1, 2, 3, 4, 5};
    int* p = arr;
    int* q = arr + 3;
    int* none = nullptr;
    void* vp = p;

    // Comparison with a null pointer constant, both ways, both spellings.
    if ((int)(p != 0) == 1 && (int)(p == 0) == 0 && (int)(0 != p) == 1 && (int)(none == 0) == 1 && (int)(none != nullptr) == 0) r += 1;

    // As conditions and in logical expressions.
    if (p && !none && (p || none) && !(none && p)) r += 1;

    // Ordering and difference within one array.
    if (p < q && q > p && p <= p && q - p == 3 && p - q == -3 && *(q - 1) == 3 && q[-2] == 2) r += 1;

    // Through void* and back, and through an integer.
    unsigned long long bits = (unsigned long long)vp;
    int* back = (int*)bits;
    if (back == p && *back == 1 && (int*)vp == p && bits % 4 == 0) r += 1;

    // A pointer to a class member, and a pointer's own address.
    T t{7};
    T* pt = &t;
    int* pv = &pt->v;
    int** ppv = &pv;
    if (*pv == 7 && **ppv == 7 && (void*)pv == (void*)pt) r += 1;

    // Pointer increments and a loop to the end.
    int sum = 0;
    for (int* it = arr; it != arr + 5; ++it) sum += *it;
    int* end = arr + 5;
    int steps = 0;
    while (p != end) { ++p; ++steps; }
    if (sum == 15 && steps == 5 && p == end) r += 1;

    return r; // six checks
}
