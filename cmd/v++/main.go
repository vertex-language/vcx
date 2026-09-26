package main

import (
	"os"
	"os/signal"
	"runtime/debug"
	"runtime/pprof"
	"syscall"

	"github.com/vertex-language/vcx/internal/cli"
)

func main() { os.Exit(run()) }

// run is cli.Run, under a CPU profile when VCX_CPUPROFILE names a file
// to write one to -- for finding where a slow build's time goes. A
// SIGTERM ends the build with the profile written, which is how a build
// that runs away gets profiled at all.
func run() int {
	// A compile allocates fast and keeps little, so collecting at twice
	// the live heap rather than once trades a little memory for less GC:
	// 7-15% off a build, measured. GOGC in the environment still decides.
	if os.Getenv("GOGC") == "" {
		debug.SetGCPercent(200)
	}
	if path := os.Getenv("VCX_CPUPROFILE"); path != "" {
		if f, err := os.Create(path); err == nil {
			defer f.Close()
			if pprof.StartCPUProfile(f) == nil {
				defer pprof.StopCPUProfile()
				term := make(chan os.Signal, 1)
				signal.Notify(term, syscall.SIGTERM)
				go func() {
					<-term
					pprof.StopCPUProfile()
					f.Close()
					os.Exit(143)
				}()
			}
		}
	}
	return cli.Run(os.Args[1:], os.Stdout, os.Stderr)
}
