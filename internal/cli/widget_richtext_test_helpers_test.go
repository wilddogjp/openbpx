package cli

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"testing"
)

func buildRichTextWidgetTestAsset(t *testing.T) (string, string) {
	return buildWidgetTestAsset(t, "canvaspanel", "CanvasPanel_21", "richtextblock", "RichTextBlock_1")
}

func buildWidgetTestAsset(t *testing.T, rootType, rootName, childType, childName string) (string, string) {
	t.Helper()

	outPath := filepath.Join(t.TempDir(), "WBP_WidgetTest.uasset")
	childPath := rootName + "/" + childName

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	for _, argv := range [][]string{
		{
			"blueprint", "widget-init", outPath,
			"--template", "minimum",
			"--package-path", "/Game/UI",
		},
		{
			"blueprint", "widget-add", outPath,
			"--parent", "root",
			"--type", rootType,
			"--name", rootName,
		},
		{
			"blueprint", "widget-add", outPath,
			"--parent", rootName,
			"--type", childType,
			"--name", childName,
		},
	} {
		stdout.Reset()
		stderr.Reset()
		if code := Run(argv, &stdout, &stderr); code != 0 {
			t.Fatalf("setup exit code: got %d want 0 argv=%v stderr=%s", code, argv, stderr.String())
		}
	}

	return outPath, childPath
}

func buildBPXBinaryForTest(t *testing.T) string {
	t.Helper()

	outPath := filepath.Join(t.TempDir(), "bpx-test-bin")
	cmd := exec.Command("go", "build", "-o", outPath, "../../cmd/bpx")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build bpx binary: %v\n%s", err, string(output))
	}
	return outPath
}

func buildRichTextWidgetTestAssetWithBinary(t *testing.T, binaryPath string) (string, string) {
	return buildWidgetTestAssetWithBinary(t, binaryPath, "canvaspanel", "CanvasPanel_21", "richtextblock", "RichTextBlock_1")
}

func buildWidgetTestAssetWithBinary(t *testing.T, binaryPath string, rootType, rootName, childType, childName string) (string, string) {
	t.Helper()

	outPath := filepath.Join(t.TempDir(), "WBP_WidgetTest.uasset")
	childPath := rootName + "/" + childName

	for _, argv := range [][]string{
		{
			"blueprint", "widget-init", outPath,
			"--template", "minimum",
			"--package-path", "/Game/UI",
		},
		{
			"blueprint", "widget-add", outPath,
			"--parent", "root",
			"--type", rootType,
			"--name", rootName,
		},
		{
			"blueprint", "widget-add", outPath,
			"--parent", rootName,
			"--type", childType,
			"--name", childName,
		},
	} {
		runBPXBinaryCommand(t, binaryPath, argv...)
	}

	return outPath, childPath
}

func runBPXBinaryCommand(t *testing.T, binaryPath string, args ...string) {
	t.Helper()
	cmd := exec.Command(binaryPath, args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("run bpx binary argv=%v: %v\n%s", args, err, string(output))
	}
}
