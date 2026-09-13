package sema_test

// The check corpus: does it typecheck, and does it say something when it
// does not?
//
// tests/check holds two kinds of file and asks the opposite question of
// each. `ok-*` must check clean: not one diagnostic, because a warning on
// correct code is a bug in the same way an error is. Everything else must be
// rejected, and each `bad-*` file is one violation with the paragraph it
// breaks written above it.
//
// What a rejection *says* is not asserted here, and that is deliberate. An
// expected-message file would have to be maintained by hand and would be
// wrong the moment a diagnostic was improved, which is a reason to leave the
// wording alone -- exactly backwards. The file itself is the record instead:
//
//	v++ check tests/check/bad-07-private-member.cpp
//
// prints the diagnostic, and reading that against the paragraph quoted in
// the file is how the wording is reviewed. The suite's job is the part that
// can be checked mechanically: that a violation is caught at all, and that
// correct code stays quiet.
//
// The oracle problem is real and has no local answer. cl.exe is the only C++
// compiler on the machine this was written on and it is not a reliable
// oracle for C++23 core-language questions -- it rejects P2223 splices,
// which GCC and Clang have accepted for years. So the citation in each file
// stands in for the second compiler, and the day one is installed the `ok-*`
// half can be handed to it unchanged: every file here is portable C++23 with
// no headers and no entry point.

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckCorpus(t *testing.T) {
	files, err := filepath.Glob("../tests/check/*.cpp")
	if err != nil || len(files) == 0 {
		t.Fatal("no files found in tests/check/*.cpp")
	}

	for _, file := range files {
		base := filepath.Base(file)
		name := strings.TrimSuffix(base, ".cpp")
		t.Run(name, func(t *testing.T) {
			diags := analyzeFile(t, file)
			errs := errorsOnly(diags)

			if strings.HasPrefix(base, "ok-") {
				if len(diags) > 0 {
					t.Errorf("a file that must check clean produced %d diagnostic(s):%s",
						len(diags), render(diags))
				}
				return
			}

			if len(errs) == 0 {
				t.Errorf("a file that must be rejected was accepted; the violation it "+
					"was written for is no longer diagnosed:\n  %s", base)
			}
		})
	}
}
