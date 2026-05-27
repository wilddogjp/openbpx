package cli

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/wilddogjp/openbpx/pkg/edit"
	"github.com/wilddogjp/openbpx/pkg/uasset"
)

type widgetCompileArtifactsSnapshot struct {
	softObjectPathOrder []string
	functionDocOrder    []string
}

func widgetCompileArtifactsReversedOrder() []string {
	return []string{"PreConstruct", "Construct", "Tick"}
}

func finalizeWidgetBlueprintMutation(originalAsset, asset *uasset.Asset, opts uasset.ParseOptions, blueprintObjectName string) ([]byte, *uasset.Asset, error) {
	if asset == nil {
		return nil, nil, fmt.Errorf("asset is nil")
	}

	workingAsset := asset
	_ = blueprintObjectName
	var indexRemap map[int32]int32
	if originalAsset != nil && len(originalAsset.Names) > 0 && len(workingAsset.Names) > 0 {
		remap, err := edit.BuildNameIndexRemapAllowInsertedNewEntries(originalAsset.Names, workingAsset.Names)
		if err == nil {
			indexRemap = remap
		}
	}
	var err error
	if len(indexRemap) > 0 && originalAsset != nil {
		_, workingAsset, err = rewriteBlueprintEditorSearchTailVerboseRecordFieldsFromOriginal(originalAsset, workingAsset, opts, indexRemap)
		if err != nil {
			return nil, nil, fmt.Errorf("rewrite blueprint editor search tail verbose fields: %w", err)
		}
	}
	outBytes, workingAsset, err := syncBlueprintEditorSearchTailOffsets(workingAsset, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("sync blueprint editor search tail: %w", err)
	}
	if originalAsset != nil {
		outBytes, workingAsset, err = patchBlueprintSearchTailIsDesignTimeBooleanVariantFromOriginal(originalAsset, workingAsset, opts)
		if err != nil {
			return nil, nil, fmt.Errorf("patch blueprint search tail IsDesignTime boolean variant: %w", err)
		}
	}
	return outBytes, workingAsset, nil
}

func captureWidgetCompileArtifactsSnapshot(asset *uasset.Asset, blueprintObjectName string) widgetCompileArtifactsSnapshot {
	if asset == nil {
		return widgetCompileArtifactsSnapshot{}
	}
	return widgetCompileArtifactsSnapshot{
		softObjectPathOrder: widgetBlueprintSoftObjectPathOrder(asset, blueprintObjectName),
		functionDocOrder:    widgetBlueprintFunctionDocBlockOrder(asset),
	}
}

func restoreWidgetBlueprintCompileArtifacts(asset *uasset.Asset, opts uasset.ParseOptions, blueprintObjectName string, snapshot widgetCompileArtifactsSnapshot) ([]byte, *uasset.Asset, error) {
	if asset == nil {
		return nil, nil, fmt.Errorf("asset is nil")
	}

	workingAsset := asset
	if len(snapshot.softObjectPathOrder) == 3 {
		var err error
		_, workingAsset, err = rewriteWidgetBlueprintSoftObjectPathOrder(workingAsset, opts, blueprintObjectName, snapshot.softObjectPathOrder)
		if err != nil {
			return nil, nil, err
		}
	}

	desiredDocOrder := snapshot.functionDocOrder
	if len(desiredDocOrder) != 3 {
		desiredDocOrder = snapshot.softObjectPathOrder
	}
	if len(desiredDocOrder) == 3 {
		var err error
		_, workingAsset, err = rewriteWidgetFunctionDocBlocks(workingAsset, opts, desiredDocOrder)
		if err != nil {
			return nil, nil, err
		}
	}

	return append([]byte(nil), workingAsset.Raw.Bytes...), workingAsset, nil
}

func normalizeRichTextWidgetCompileArtifacts(asset *uasset.Asset, opts uasset.ParseOptions, className, blueprintObjectName string) ([]byte, *uasset.Asset, error) {
	if asset == nil {
		return nil, nil, fmt.Errorf("asset is nil")
	}
	if !strings.EqualFold(strings.TrimSpace(className), "RichTextBlock") {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}
	return restoreWidgetBlueprintCompileArtifacts(asset, opts, blueprintObjectName, widgetCompileArtifactsSnapshot{
		softObjectPathOrder: widgetCompileArtifactsReversedOrder(),
		functionDocOrder:    widgetCompileArtifactsReversedOrder(),
	})
}

func ensureWidgetTextPackageFlags(asset *uasset.Asset, opts uasset.ParseOptions, normalizedProperty string) ([]byte, *uasset.Asset, error) {
	if asset == nil {
		return nil, nil, fmt.Errorf("asset is nil")
	}
	switch strings.ToLower(strings.TrimSpace(normalizedProperty)) {
	case "text",
		"editabletext-hint-text", "editabletexthinttext",
		"editabletextbox-hint-text", "editabletextboxhinttext",
		"multilineeditabletextbox-hint-text", "multilineeditabletextboxhinttext":
	default:
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}
	newFlags := asset.Summary.PackageFlags | packageFlagRequiresLoc
	if newFlags == asset.Summary.PackageFlags {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}
	outBytes, _, err := rewritePackageFlags(asset, newFlags)
	if err != nil {
		return nil, nil, err
	}
	updatedAsset, err := uasset.ParseBytes(outBytes, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("reparse package flags rewrite: %w", err)
	}
	return outBytes, updatedAsset, nil
}

func widgetBlueprintSoftObjectPathOrder(asset *uasset.Asset, blueprintObjectName string) []string {
	if asset == nil {
		return nil
	}
	trimmedBlueprint := strings.TrimSpace(blueprintObjectName)
	if trimmedBlueprint == "" {
		return nil
	}

	entries, err := edit.ReadSoftObjectPathEntries(asset)
	if err != nil {
		return nil
	}

	targetAsset := "SKEL_" + trimmedBlueprint + "_C"
	seen := map[string]bool{
		"PreConstruct": false,
		"Construct":    false,
		"Tick":         false,
	}
	order := make([]string, 0, 3)
	for i, entry := range entries {
		_ = i
		if !strings.EqualFold(entry.AssetName.Display(asset.Names), targetAsset) {
			continue
		}
		if _, ok := seen[entry.SubPath]; !ok {
			continue
		}
		if seen[entry.SubPath] {
			continue
		}
		seen[entry.SubPath] = true
		order = append(order, entry.SubPath)
		if len(order) == 3 {
			return order
		}
	}
	return nil
}

func rewriteWidgetBlueprintSoftObjectPathOrder(asset *uasset.Asset, opts uasset.ParseOptions, blueprintObjectName string, desiredOrder []string) ([]byte, *uasset.Asset, error) {
	if asset == nil {
		return nil, nil, fmt.Errorf("asset is nil")
	}
	if len(desiredOrder) != 3 {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}
	trimmedBlueprint := strings.TrimSpace(blueprintObjectName)
	if trimmedBlueprint == "" {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}

	entries, err := edit.ReadSoftObjectPathEntries(asset)
	if err != nil {
		return nil, nil, err
	}

	targetPackage := strings.TrimSpace(asset.Summary.PackageName)
	targetAsset := "SKEL_" + trimmedBlueprint + "_C"
	positions := map[string]int{
		"PreConstruct": -1,
		"Construct":    -1,
		"Tick":         -1,
	}
	for i, entry := range entries {
		if !strings.EqualFold(entry.PackageName.Display(asset.Names), targetPackage) {
			continue
		}
		if !strings.EqualFold(entry.AssetName.Display(asset.Names), targetAsset) {
			continue
		}
		if _, ok := positions[entry.SubPath]; !ok {
			continue
		}
		positions[entry.SubPath] = i
	}

	targetPositions := make([]int, 0, len(positions))
	for _, subPath := range desiredOrder {
		pos, ok := positions[subPath]
		if !ok || pos < 0 {
			return append([]byte(nil), asset.Raw.Bytes...), asset, nil
		}
		targetPositions = append(targetPositions, pos)
	}
	sortIntsLocal(targetPositions)

	reordered := make([]edit.SoftObjectPathEntry, len(entries))
	copy(reordered, entries)
	for i, subPath := range desiredOrder {
		reordered[targetPositions[i]] = entries[positions[subPath]]
	}
	if reordered[targetPositions[0]] == entries[targetPositions[0]] &&
		reordered[targetPositions[1]] == entries[targetPositions[1]] &&
		reordered[targetPositions[2]] == entries[targetPositions[2]] {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}

	outBytes, err := edit.RewriteSoftObjectPathEntries(asset, reordered)
	if err != nil {
		return nil, nil, err
	}
	updatedAsset, err := uasset.ParseBytes(outBytes, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("reparse soft object path rewrite: %w", err)
	}
	return outBytes, updatedAsset, nil
}

func widgetBlueprintFunctionDocBlockOrder(asset *uasset.Asset) []string {
	if asset == nil {
		return nil
	}

	region, _, _, ok := widgetFunctionDocBlockRegion(asset)
	if !ok {
		return nil
	}
	clusters, ok := widgetFunctionDocBlockClusters(region)
	if !ok || len(clusters) == 0 || len(clusters[0]) != 3 {
		return nil
	}

	order := make([]string, 0, len(clusters[0]))
	for _, block := range clusters[0] {
		switch {
		case bytes.Contains(block.data, []byte("Ticks this widget.")):
			order = append(order, "Tick")
		case bytes.Contains(block.data, []byte("Called after the underlying slate widget is constructed.")):
			order = append(order, "Construct")
		case bytes.Contains(block.data, []byte("Called by both the game and the editor.")):
			order = append(order, "PreConstruct")
		default:
			return nil
		}
	}
	return order
}

func rewriteWidgetFunctionDocBlocks(asset *uasset.Asset, opts uasset.ParseOptions, desiredOrder []string) ([]byte, *uasset.Asset, error) {
	if asset == nil {
		return nil, nil, fmt.Errorf("asset is nil")
	}
	if len(desiredOrder) != 3 {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}

	region, regionStart, regionEnd, ok := widgetFunctionDocBlockRegion(asset)
	if !ok {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}
	clusters, ok := widgetFunctionDocBlockClusters(region)
	if !ok || len(clusters) == 0 {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}

	rewrittenRegion := make([]byte, 0, len(region))
	cursor := 0
	for _, cluster := range clusters {
		if len(cluster) != 3 {
			return append([]byte(nil), asset.Raw.Bytes...), asset, nil
		}
		rewrittenRegion = append(rewrittenRegion, region[cursor:cluster[0].start]...)
		blockByKind := make(map[string][]byte, len(cluster))
		for _, block := range cluster {
			switch {
			case bytes.Contains(block.data, []byte("Ticks this widget.")):
				blockByKind["Tick"] = append([]byte(nil), block.data...)
			case bytes.Contains(block.data, []byte("Called after the underlying slate widget is constructed.")):
				blockByKind["Construct"] = append([]byte(nil), block.data...)
			case bytes.Contains(block.data, []byte("Called by both the game and the editor.")):
				blockByKind["PreConstruct"] = append([]byte(nil), block.data...)
			}
		}
		if len(blockByKind) != 3 {
			return append([]byte(nil), asset.Raw.Bytes...), asset, nil
		}
		for ordinal, kind := range desiredOrder {
			block, ok := blockByKind[kind]
			if !ok {
				return append([]byte(nil), asset.Raw.Bytes...), asset, nil
			}
			rewrittenRegion = append(rewrittenRegion, patchWidgetFunctionDocBlockOrderIndex(block, ordinal)...)
		}
		cursor = cluster[len(cluster)-1].end
	}
	rewrittenRegion = append(rewrittenRegion, region[cursor:]...)
	if bytes.Equal(rewrittenRegion, region) {
		return append([]byte(nil), asset.Raw.Bytes...), asset, nil
	}

	outBytes, err := edit.RewriteRawRange(asset, int64(regionStart), int64(regionEnd), rewrittenRegion)
	if err != nil {
		return nil, nil, fmt.Errorf("rewrite function doc block order: %w", err)
	}
	updatedAsset, err := uasset.ParseBytes(outBytes, opts)
	if err != nil {
		return nil, nil, fmt.Errorf("reparse function doc block rewrite: %w", err)
	}
	return outBytes, updatedAsset, nil
}

type widgetFunctionDocBlock struct {
	start int
	end   int
	data  []byte
}

func widgetFunctionDocBlockRegion(asset *uasset.Asset) ([]byte, int, int, bool) {
	if asset == nil {
		return nil, 0, 0, false
	}
	regionStart := int(asset.Summary.SoftObjectPathsOffset)
	regionEnd := int(asset.Summary.ImportOffset)
	if regionStart < 0 || regionEnd <= regionStart || regionEnd > len(asset.Raw.Bytes) {
		return nil, 0, 0, false
	}
	return asset.Raw.Bytes[regionStart:regionEnd], regionStart, regionEnd, true
}

func widgetFunctionDocBlockClusters(region []byte) ([][]widgetFunctionDocBlock, bool) {
	blockMarker := []byte("User Interface\x00")
	headerLen := 20
	searchFrom := 0
	clusters := make([][]widgetFunctionDocBlock, 0, 2)
	for {
		blockStarts := make([]int, 0, 3)
		for len(blockStarts) < 3 {
			rel := bytes.Index(region[searchFrom:], blockMarker)
			if rel < 0 {
				if len(clusters) == 0 {
					return nil, false
				}
				return clusters, true
			}
			start := searchFrom + rel
			if start >= headerLen {
				start -= headerLen
			}
			blockStarts = append(blockStarts, start)
			searchFrom += rel + len(blockMarker)
		}

		trailerRel := bytes.Index(region[blockStarts[2]:], []byte("true\x00"))
		if trailerRel < 0 {
			return nil, false
		}
		trailerStart := blockStarts[2] + trailerRel
		if trailerStart >= headerLen {
			trailerStart -= headerLen
		}
		blockEnds := []int{blockStarts[1], blockStarts[2], trailerStart}

		cluster := make([]widgetFunctionDocBlock, 0, len(blockStarts))
		for i := range blockStarts {
			cluster = append(cluster, widgetFunctionDocBlock{
				start: blockStarts[i],
				end:   blockEnds[i],
				data:  append([]byte(nil), region[blockStarts[i]:blockEnds[i]]...),
			})
		}
		clusters = append(clusters, cluster)
		searchFrom = trailerStart
	}
}

func patchWidgetFunctionDocBlockOrderIndex(block []byte, ordinal int) []byte {
	if len(block) < 4 || ordinal < 0 || ordinal > 0xff {
		return block
	}
	out := append([]byte(nil), block...)
	out[0] = byte(ordinal)
	out[1] = 0
	out[2] = 0
	out[3] = 0
	return out
}
