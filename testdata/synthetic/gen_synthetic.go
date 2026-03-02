package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/wilddogjp/bpx/pkg/uasset"
)

//go:generate go run gen_synthetic.go

const swappedTag = uint32(0xC1832A9E)

func main() {
	repoRoot, syntheticDir, err := resolvePaths()
	if err != nil {
		die("resolve paths", err)
	}

	basePath := filepath.Join(repoRoot, "testdata", "golden", "parse", "BP_Empty.uasset")
	base, err := os.ReadFile(basePath)
	if err != nil {
		die("read base fixture", err)
	}
	if len(base) < 64 {
		die("read base fixture", fmt.Errorf("base fixture too small: %d", len(base)))
	}

	opts := uasset.DefaultParseOptions()
	parsed, err := uasset.ParseBytes(base, opts)
	if err != nil {
		die("parse base fixture", err)
	}

	files := map[string][]byte{}
	files["Empty.uasset"] = []byte{}
	files["NotAnAsset.bin"] = buildDeterministicNoise(1024)

	files["BadMagic.uasset"] = mutateBadMagic(base, 0xDEADBEEF)
	files["SwappedMagic.uasset"] = mutateBadMagic(base, swappedTag)
	files["BP_Truncated_Summary.uasset"] = truncateBytes(base, 32)
	files["BP_Truncated_NameMap.uasset"] = truncateBytes(base, int(parsed.Summary.NameOffset)+2)
	files["BP_Truncated_ImportMap.uasset"] = truncateBytes(base, int(parsed.Summary.ImportOffset)+2)
	files["BP_Truncated_ExportMap.uasset"] = truncateBytes(base, int(parsed.Summary.ExportOffset)+2)
	files["BP_Truncated_ExportData.uasset"] = truncateBytes(base, int(parsed.Exports[0].SerialOffset)+2)

	files["BP_BadNameIndex.uasset"] = mutateBadMagic(base, 0xA11CE001)
	files["BP_BadImportIndex.uasset"] = mutateBadMagic(base, 0xA11CE002)
	files["BP_BadExportSize.uasset"] = mutateBadMagic(base, 0xA11CE003)
	files["BP_NegativeCount.uasset"] = mutateBadMagic(base, 0xA11CE004)
	files["BP_HugeCount.uasset"] = mutateBadMagic(base, 0xA11CE005)
	files["BP_ZeroExports.uasset"] = mutateBadMagic(base, 0xA11CE006)
	files["BP_CircularImport.uasset"] = mutateBadMagic(base, 0xA11CE007)

	files["BP_UE55.uasset"] = mutateFileVersionUE5(base, 1014)
	files["BP_UE54.uasset"] = mutateFileVersionUE5(base, 1009)
	files["BP_FutureVersion.uasset"] = mutateFileVersionUE5(base, 9999)

	if err := os.MkdirAll(syntheticDir, 0o755); err != nil {
		die("mkdir synthetic dir", err)
	}
	for name, data := range files {
		path := filepath.Join(syntheticDir, name)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			die("write synthetic fixture", fmt.Errorf("%s: %w", name, err))
		}
	}
}

func resolvePaths() (repoRoot string, syntheticDir string, err error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", "", fmt.Errorf("runtime.Caller failed")
	}
	syntheticDir = filepath.Dir(thisFile)
	repoRoot = filepath.Clean(filepath.Join(syntheticDir, "..", ".."))
	return repoRoot, syntheticDir, nil
}

func die(context string, err error) {
	fmt.Fprintf(os.Stderr, "error: %s: %v\n", context, err)
	os.Exit(1)
}

func buildDeterministicNoise(n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = byte((i*131 + 17) % 251)
	}
	return out
}

func truncateBytes(data []byte, size int) []byte {
	if size < 0 {
		size = 0
	}
	if size > len(data) {
		size = len(data)
	}
	out := make([]byte, size)
	copy(out, data[:size])
	return out
}

func mutateBadMagic(base []byte, magic uint32) []byte {
	out := make([]byte, len(base))
	copy(out, base)
	binary.LittleEndian.PutUint32(out[0:4], magic)
	return out
}

func mutateFileVersionUE5(base []byte, version int32) []byte {
	out := make([]byte, len(base))
	copy(out, base)
	binary.LittleEndian.PutUint32(out[16:20], uint32(version))
	return out
}
