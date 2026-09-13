package vcx

// Build compiles and links a program to output.
func Build(output string, sources ...string) error {
	var inputs []Input
	for _, s := range sources {
		inputs = append(inputs, File(s))
	}
	c := &Compiler{}
	return c.Build(BuildParams{
		Output: output,
		Inputs: inputs,
	})
}

// Run compiles, links and runs a source file.
func Run(source string) ([]byte, error) {
	c := &Compiler{}
	return c.Run(source)
}

// Check parses and checks a source file.
func Check(source string) ([]Diagnostic, error) {
	c := &Compiler{}
	return c.Check(File(source))
}
