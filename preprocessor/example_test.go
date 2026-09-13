package preprocessor_test

import (
	"fmt"

	"github.com/vertex-language/vcx/preprocessor"
	"github.com/vertex-language/vcx/token"
)

// A dependency scan is a phase-4 output: what this unit provides, and what it
// requires. No parsing, no BMIs, no build system.
func ExamplePreprocessor_Modules() {
	src := []byte("module;\nexport module app.core;\nimport std;\nimport <vector>;\nexport import :api;\n")
	p := preprocessor.New(preprocessor.Config{})
	p.Run(token.NewFile("core.cppm", src))

	for _, d := range p.Modules() {
		switch d.Kind {
		case preprocessor.Interface:
			fmt.Println("provides", d.Name)
		case preprocessor.Import:
			fmt.Println("requires", d.Name)
		case preprocessor.ImportHeader:
			fmt.Println("requires header", d.Header)
		}
	}
	// Output:
	// provides app.core
	// requires std
	// requires header vector
	// requires :api
}
