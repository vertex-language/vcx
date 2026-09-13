package parser

import (
	"testing"

	"github.com/vertex-language/vcx/ast"
	"github.com/vertex-language/vcx/token"
)

func parseString(t *testing.T, src string) (*ast.File, []Diagnostic) {
	t.Helper()
	f := token.NewFile("test.cpp", []byte(src))
	return ParseFile(f, DefaultMode)
}

func mustParse(t *testing.T, src string) *ast.File {
	t.Helper()
	file, diags := parseString(t, src)
	for _, d := range diags {
		if d.Severity == token.Error {
			t.Fatalf("unexpected parse error: %v\nSource:\n%s", d, src)
		}
	}
	return file
}

func TestParseSimpleDeclarations(t *testing.T) {
	src := `
int x = 42;
double y = 3.14;
const char* msg = "hello";
void f();
int add(int a, int b) {
    return a + b;
}
`
	file := mustParse(t, src)
	if len(file.Decls) != 5 {
		t.Fatalf("expected 5 declarations, got %d", len(file.Decls))
	}
}

func TestParseClassHierarchy(t *testing.T) {
	src := `
class Base {
public:
    virtual ~Base() = default;
    virtual void run() = 0;
protected:
    int id;
};

class Derived : public Base {
public:
    Derived(int x) : id(x) {}
    void run() override {}
private:
    int extra = 0;
};
`
	file := mustParse(t, src)
	if len(file.Decls) != 2 {
		t.Fatalf("expected 2 declarations, got %d", len(file.Decls))
	}
}

func TestParseTemplatesAndConcepts(t *testing.T) {
	src := `
template <typename T>
concept Hashable = requires(T a) {
    { hash(a) } -> std::same_as<size_t>;
};

template <typename T>
requires Hashable<T>
class Set {
public:
    void insert(const T& val);
};

template <typename T, int N = 10>
T max(T a, T b) {
    return a > b ? a : b;
}

template <>
int max<int, 10>(int a, int b) {
    return a > b ? a : b;
}
`
	file := mustParse(t, src)
	if len(file.Decls) < 4 {
		t.Fatalf("expected at least 4 declarations, got %d", len(file.Decls))
	}
}

func TestParseLambdasAndFolds(t *testing.T) {
	src := `
void test() {
    auto l1 = [](int x, int y) { return x + y; };
    auto l2 = [&, x = 10]<typename T>(T val) mutable -> int { return val + x; };
}

template <typename... Args>
auto sum(Args... args) {
    return (args + ...);
}

template <typename... Args>
bool all(Args... args) {
    return (... && args);
}
`
	mustParse(t, src)
}

func TestParseStructuredBindingAndRangeFor(t *testing.T) {
	src := `
void test() {
    auto [x, y] = get_pair();
    for (auto&& [k, v] : map) {
        process(k, v);
    }
}
`
	mustParse(t, src)
}

func TestParseCoroutines(t *testing.T) {
	src := `
Task<int> compute() {
    co_await ready();
    co_yield 42;
    co_return 100;
}
`
	mustParse(t, src)
}

func TestParseModules(t *testing.T) {
	src := `
export module my_lib.core;
import <vector>;
import <string>;

export int get_version() {
    return 1;
}
`
	file := mustParse(t, src)
	if file.Module == nil {
		t.Fatalf("expected module declaration to be recorded on ast.File")
	}
}

func TestParseStatementsAndControlFlow(t *testing.T) {
	src := `
void test(int n) {
    if (int x = n; x > 0) {
        do_positive();
    } else if constexpr (false) {
        do_unreachable();
    } else {
        do_zero();
    }

    switch (n) {
    case 1:
        break;
    case 2 ... 5:
        break;
    default:
        return;
    }

    while (n > 0) {
        --n;
        if (n == 2) continue;
    }

    do {
        n++;
    } while (n < 5);

    try {
        risky();
    } catch (const std::exception& e) {
        log(e);
    } catch (...) {
        fallback();
    }
}
`
	mustParse(t, src)
}

func TestParseTypeTraitsAndCasts(t *testing.T) {
	src := `
void test() {
    bool b = __is_base_of(Base, Derived);
    int* p = static_cast<int*>(ptr);
    Base* bp = dynamic_cast<Base*>(dp);
    const int* cp = const_cast<const int*>(p);
    long addr = reinterpret_cast<long>(p);
    int c_cast = (int)3.14;
    auto s = sizeof(int);
    auto sp = sizeof...(Args);
    auto al = alignof(double);
}
`
	mustParse(t, src)
}

func TestParseSyntaxErrorRecovery(t *testing.T) {
	src := `
int valid1 = 1;
int error_var = ;
int valid2 = 2;
void func( {
}
int valid3 = 3;
`
	file, diags := parseString(t, src)
	if len(diags) == 0 {
		t.Fatalf("expected parse diagnostics for syntax errors")
	}
	// Verify that valid declarations survived recovery
	if len(file.Decls) < 3 {
		t.Errorf("expected recovery to preserve valid declarations, got %d decls", len(file.Decls))
	}
}
