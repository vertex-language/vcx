// Package offload is the host side of a CUDA or HIP program: the
// containers a device image travels in inside the host object, which
// the runtime's registration reads back at startup.
//
// The formats are the vendors': NVIDIA's fat binary, which cudart's
// __cudaRegisterFatBinary takes, and the clang offload bundle HIP's
// __hipRegisterFatBinary takes. vcx's own runtime reads both, and so
// does the vendor's, which is what lets a program built without a
// toolkit link against the real one later.
package offload

import (
	"encoding/binary"
)

// An Image is one device image for one architecture.
type Image struct {
	// Kind is what the bytes are: PTX text or an ELF code object.
	Kind ImageKind

	// Arch is the architecture the image is for: sm_75, gfx942.
	Arch string

	// SM is the NVIDIA compute capability as a number, 75 for sm_75.
	SM int

	// ISA is the PTX ISA version of a PTX image: major and minor.
	ISAMajor, ISAMinor int

	// Data is the image itself. A PTX image is text without a
	// terminating NUL; the container adds one.
	Data []byte

	// Host is the operating system the program is built for, which the
	// entry's flags record: "windows", "linux" or "macos".
	Host string
}

// ImageKind is the form of a device image.
type ImageKind uint8

const (
	PTX ImageKind = iota + 1
	ELF
)

// The fat binary container, as fatbinary.exe of CUDA 13 lays it out
// (read back with the same tool's cuobjdump): a sixteen-byte header and,
// for each image, an eighty-byte entry followed by its payload, the
// payload padded to eight. The entry's fields past the obvious ones
// are what the tool writes for an uncompressed image; the two words at
// 20 and 64 are copied as observed, since nothing documents them.
const (
	fatbinMagic   = 0xBA55ED50
	fatbinVersion = 1

	fatbinKindPTX = 1
	fatbinKindELF = 2

	// The entry flags: the image is for a 64-bit host, and which host.
	fatbinFlag64Bit   = 0x01
	fatbinFlagLinux   = 0x10
	fatbinFlagMac     = 0x20
	fatbinFlagWindows = 0x40
)

// Fatbin encodes the images as an NVIDIA fat binary.
func Fatbin(images []Image) []byte {
	var body []byte
	for _, im := range images {
		payload := im.Data
		if im.Kind == PTX {
			payload = append(append([]byte(nil), im.Data...), 0)
		}
		padded := (len(payload) + 7) &^ 7
		entry := make([]byte, 80)
		le := binary.LittleEndian
		kind := uint16(fatbinKindELF)
		if im.Kind == PTX {
			kind = fatbinKindPTX
		}
		flags := uint64(fatbinFlag64Bit)
		switch im.Host {
		case "linux":
			flags |= fatbinFlagLinux
		case "macos", "darwin":
			flags |= fatbinFlagMac
		default:
			flags |= fatbinFlagWindows
		}
		le.PutUint16(entry[0:], kind)
		le.PutUint16(entry[2:], 0x0101)
		le.PutUint32(entry[4:], 80)
		le.PutUint64(entry[8:], uint64(padded))
		le.PutUint32(entry[16:], 0) // compressed size: none
		le.PutUint32(entry[20:], 64)
		le.PutUint16(entry[24:], uint16(im.ISAMinor))
		le.PutUint16(entry[26:], uint16(im.ISAMajor))
		le.PutUint32(entry[28:], uint32(im.SM))
		le.PutUint32(entry[32:], 0) // object name offset
		le.PutUint32(entry[36:], 0) // object name length
		le.PutUint64(entry[40:], flags)
		le.PutUint64(entry[48:], 0)
		le.PutUint64(entry[56:], 0) // decompressed size: not compressed
		le.PutUint32(entry[64:], 0x48)
		body = append(body, entry...)
		body = append(body, payload...)
		for len(body)%8 != 0 {
			body = append(body, 0)
		}
	}
	header := make([]byte, 16)
	binary.LittleEndian.PutUint32(header[0:], fatbinMagic)
	binary.LittleEndian.PutUint16(header[4:], fatbinVersion)
	binary.LittleEndian.PutUint16(header[6:], 16)
	binary.LittleEndian.PutUint64(header[8:], uint64(len(body)))
	return append(header, body...)
}

// The fat binary wrapper: the four words __cudaRegisterFatBinary is
// handed a pointer to. The magic says it is one; the version is 1; the
// data is the fat binary; the last word is unused.
const (
	FatbinWrapperMagic   = 0x466243B1
	FatbinWrapperVersion = 1
)

// The clang offload bundle, which HIP's registration takes: a magic, the
// entry count, and for each entry the offset and size of its image and
// the target id it is for, followed by the images.
const bundleMagic = "__CLANG_OFFLOAD_BUNDLE__"

// Bundle encodes the images as a clang offload bundle for HIP, with the
// host entry the runtime expects first.
func Bundle(images []Image) []byte {
	type entry struct {
		id   string
		data []byte
	}
	entries := []entry{{id: "host-x86_64-unknown-linux-gnu", data: nil}}
	for _, im := range images {
		entries = append(entries, entry{id: "hipv4-amdgcn-amd-amdhsa--" + im.Arch, data: im.Data})
	}
	le := binary.LittleEndian
	header := len(bundleMagic) + 8
	for _, e := range entries {
		header += 8 + 8 + 8 + len(e.id)
	}
	out := make([]byte, 0, header)
	out = append(out, bundleMagic...)
	out = le.AppendUint64(out, uint64(len(entries)))
	offset := (header + 4095) &^ 4095
	type placed struct{ off, size int }
	var places []placed
	for _, e := range entries {
		places = append(places, placed{offset, len(e.data)})
		out = le.AppendUint64(out, uint64(offset))
		out = le.AppendUint64(out, uint64(len(e.data)))
		out = le.AppendUint64(out, uint64(len(e.id)))
		out = append(out, e.id...)
		offset += (len(e.data) + 4095) &^ 4095
	}
	for i, e := range entries {
		for len(out) < places[i].off {
			out = append(out, 0)
		}
		out = append(out, e.data...)
	}
	for len(out)%4096 != 0 {
		out = append(out, 0)
	}
	return out
}
