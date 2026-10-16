# Linux headers vcx carries

vcx compiles C++ for a Linux target against these, embedded in the binary
(`sysroot.go`, `linuxHeaders`), so a Linux build does not depend on what the
building machine has installed:

- `c++/v1` — libc++ 20.1.2 (Ubuntu 24.04 `libc++-20-dev`, with `cxxabi.h`
  and `__cxxabi_config.h`), without `__cxx03`, which only `-std=c++03`
  reads. Used for every Linux target: a distribution's libc++ may be a
  version vcx cannot parse (LLVM 18's is one).
- `usr/include` — glibc 2.39 (`libc6-dev`, `libcrypt-dev`) and the
  kernel's UAPI headers (`linux-libc-dev` 6.8) for x86_64, with the
  multiarch directory `x86_64-linux-gnu` inside it as Debian lays it out.
  The drm, sound, rdma, xen, video and mtd trees are left out. Used when the
  building machine is not Linux; on Linux the machine's own are searched.

Licences are the upstream projects': libc++ is Apache-2.0 WITH
LLVM-exception, glibc's headers are LGPL-2.1-or-later, and the kernel's
UAPI headers are GPL-2.0 WITH Linux-syscall-note.

To refresh, on an x86_64 Ubuntu machine with those packages installed, copy
the files `dpkg -L libc6-dev linux-libc-dev libcrypt-dev` lists under
`/usr/include` (minus the trees above) into `usr/include`, and
`/usr/lib/llvm-20/include/c++/v1` minus `__cxx03` into `c++/v1`.
