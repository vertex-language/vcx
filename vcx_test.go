package vcx

import (
	"strings"
	"testing"

	"github.com/vertex-language/vcx/parser"
)

func TestCompilerParseAndCheck(t *testing.T) {
	src := `
template <typename T>
concept Integral = requires(T a) {
    a + 1;
};

template <Integral T>
T square(T x) {
    return x * x;
}

int main() {
    return square(5);
}
`
	in := Text("main.cpp", []byte(src))
	c := &Compiler{
		Std: Cxx23,
	}

	file, diags, err := c.Parse(in, parser.DefaultMode)
	if err != nil {
		t.Fatalf("compiler.Parse failed: %v", err)
	}
	defer file.Release()

	if HasErrors(diags) {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if len(file.Decls) != 3 {
		t.Errorf("expected 3 decls, got %d", len(file.Decls))
	}
}

func TestCompilerPreprocess(t *testing.T) {
	src := `
#define VALUE 42
int x = VALUE;
`
	in := Text("test.cpp", []byte(src))
	c := &Compiler{}

	out, diags, err := c.Preprocess(in)
	if err != nil {
		t.Fatalf("compiler.Preprocess failed: %v", err)
	}
	if HasErrors(diags) {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	str := string(out)
	if !strings.Contains(str, "42") {
		t.Errorf("expected macro VALUE to expand to 42, got:\n%s", str)
	}
}
