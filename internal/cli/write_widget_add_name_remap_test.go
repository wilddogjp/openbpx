package cli

import (
	"bytes"
	"testing"

	"github.com/wilddogjp/openbpx/pkg/uasset"
)

func TestEnsureWidgetAddRootPrerequisitesPreservesTickTailFString(t *testing.T) {
	t.Parallel()

	asset, err := uasset.ParseFile("../../testdata/golden/ue5.6/operations/widget_add_root_verticalbox/before.uasset", uasset.ParseOptions{KeepUnknown: true})
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}

	rawBefore := asset.Raw.Bytes[asset.Exports[3].SerialOffset : asset.Exports[3].SerialOffset+asset.Exports[3].SerialSize]
	if !bytes.Contains(rawBefore, []byte("0.0")) {
		t.Fatalf("expected baseline Tick node tail string 0.0")
	}

	_, workingAsset, _, _, err := ensureWidgetAddRootPrerequisites(asset, uasset.ParseOptions{KeepUnknown: true}, "VerticalBox", widgetAddName{
		Display: "VerticalBox_21",
		Base:    "VerticalBox",
		Number:  21,
	})
	if err != nil {
		t.Fatalf("ensure prerequisites: %v", err)
	}

	rawAfter := workingAsset.Raw.Bytes[workingAsset.Exports[3].SerialOffset : workingAsset.Exports[3].SerialOffset+workingAsset.Exports[3].SerialSize]
	if !bytes.Contains(rawAfter, []byte("0.0")) {
		t.Fatalf("expected Tick node tail string 0.0 after prerequisite rewrite")
	}
	if bytes.Contains(rawAfter, []byte("0.2")) {
		t.Fatalf("unexpected Tick node tail string 0.2 after prerequisite rewrite")
	}
}
