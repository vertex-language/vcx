package sema_test

// The eval corpus: does the constant evaluator get the right answer?
//
// tests/eval is the one corpus that runs today. Every file there is a whole
// program whose assertions are static_asserts, so compiling the file *is*
// running it: the loops iterate, the arrays are written to, the recursion
// recurses, and a wrong answer is a diagnostic rather than a number nobody
// reads. A compiler with no code generator can still be asked to execute
// C++, and this is where it is asked.
//
// Nothing here writes down an expected value in a harness, for the reason
// the whole suite avoids them: a table of expected numbers beside a program
// is a claim maintained by hand and wrong the moment it drifts. The claim
// lives in the program instead, as `static_assert(fib(20) == 6765)`, where
// it is checked by the thing it is a claim about.
//
// These files are also the corpus with a working oracle, and the only one.
// A static_assert file is portable C++23 -- no headers, no entry point, no
// platform -- so
//
//	cl /nologo /std:c++latest /Zs tests\eval\02-constexpr-functions.cpp
//
// compiles the same file and has to accept it too. That is not a comparison
// of output, because there is no output: both compilers either accept the
// file or name the assertion that failed, and agreeing on which is the whole
// question. Running it under cl.exe is not wired into the suite -- the
// machine this was written on needs a wrapper to set %INCLUDE% at all -- but
// nothing in these files stands in the way of it.
//
// One thing they do assert about the target: tests/eval/04 checks sizeof and
// alignof, which are LP64's. The runner analyzes under that model for that
// reason, and a file added here must not assume the host's.

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestEvalCorpus(t *testing.T) {
	files, err := filepath.Glob("../tests/eval/*.cpp")
	if err != nil || len(files) == 0 {
		t.Fatal("no files found in tests/eval/*.cpp")
	}

	for _, file := range files {
		name := strings.TrimSuffix(filepath.Base(file), ".cpp")
		t.Run(name, func(t *testing.T) {
			// A file here has to check clean, and a failed static_assert is
			// the interesting way for it not to: the evaluator ran the
			// program and got a different answer than the program claims.
			// An error from any earlier phase means the file stopped being
			// a program before it could be run, which is a failure of the
			// same weight and is reported with the phase that produced it.
			if diags := analyzeFile(t, file); len(diags) > 0 {
				t.Errorf("the evaluator disagreed with a program that asserts its own "+
					"answers, %d time(s):%s", len(diags), render(diags))
			}
		})
	}
}
