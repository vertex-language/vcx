//go:build !windows

package vcx_test

// driverPresent is the machine's driver, which the programs corpus needs;
// the driver harness is Windows-only so far, and elsewhere the corpus
// skips.
func driverPresent() (any, string) { return nil, "the CUDA driver harness is Windows-only so far" }
