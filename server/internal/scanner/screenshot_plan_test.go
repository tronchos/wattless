package scanner

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/go-rod/rod/lib/proto"
	"github.com/tronchos/wattless/server/internal/config"
)

func TestBuildScreenshotPlanUsesSingleTileForMediumPages(t *testing.T) {
	plan := buildScreenshotPlan(documentMetrics{
		ViewportWidth:  1440,
		ViewportHeight: 900,
		DocumentWidth:  1440,
		DocumentHeight: 3200,
	}, config.Config{
		FullPageMaxHeight:           16000,
		FullPageSingleShotThreshold: 8000,
		FullPageTileHeight:          2400,
	})

	if plan.Strategy != "single" {
		t.Fatalf("expected single strategy, got %s", plan.Strategy)
	}
	if len(plan.Tiles) != 1 {
		t.Fatalf("expected 1 tile, got %d", len(plan.Tiles))
	}
	if plan.Tiles[0].Height != 3200 {
		t.Fatalf("expected tile height 3200, got %d", plan.Tiles[0].Height)
	}
}

func TestBuildScreenshotPlanSplitsTallPagesIntoTiles(t *testing.T) {
	plan := buildScreenshotPlan(documentMetrics{
		ViewportWidth:  1440,
		ViewportHeight: 900,
		DocumentWidth:  1440,
		DocumentHeight: 9800,
	}, config.Config{
		FullPageMaxHeight:           16000,
		FullPageSingleShotThreshold: 8000,
		FullPageTileHeight:          2400,
	})

	if plan.Strategy != "tiled" {
		t.Fatalf("expected tiled strategy, got %s", plan.Strategy)
	}
	if len(plan.Tiles) != 5 {
		t.Fatalf("expected 5 tiles, got %d", len(plan.Tiles))
	}
	if plan.Tiles[1].Y != 2400 {
		t.Fatalf("expected second tile y=2400, got %d", plan.Tiles[1].Y)
	}
	if plan.Tiles[4].Height != 200 {
		t.Fatalf("expected last tile height 200, got %d", plan.Tiles[4].Height)
	}
}

func TestBuildScreenshotPlanTruncatesExtremelyLongPages(t *testing.T) {
	plan := buildScreenshotPlan(documentMetrics{
		ViewportWidth:  1440,
		ViewportHeight: 900,
		DocumentWidth:  1440,
		DocumentHeight: 24380,
	}, config.Config{
		FullPageMaxHeight:           16000,
		FullPageSingleShotThreshold: 8000,
		FullPageTileHeight:          2400,
	})

	if !plan.Truncated {
		t.Fatal("expected truncated plan")
	}
	if plan.CapturedHeight != 16000 {
		t.Fatalf("expected captured height 16000, got %d", plan.CapturedHeight)
	}
	if plan.Tiles[len(plan.Tiles)-1].Y >= plan.CapturedHeight {
		t.Fatalf("expected last tile start below captured height, got %d", plan.Tiles[len(plan.Tiles)-1].Y)
	}
}

func TestComposeDocumentMetricsPrefersExpandedDOMHeight(t *testing.T) {
	metrics := composeDocumentMetrics(layoutDocumentMetrics{
		ContentWidth:  1440,
		ContentHeight: 900,
		LayoutWidth:   1440,
		LayoutHeight:  900,
	}, domDocumentMetrics{
		DocumentWidth:  1440,
		DocumentHeight: 5760,
	}, config.Config{
		ViewportWidth:  1440,
		ViewportHeight: 900,
	})

	if metrics.DocumentHeight != 5760 {
		t.Fatalf("expected expanded DOM height to win, got %d", metrics.DocumentHeight)
	}
	if metrics.ViewportHeight != 900 {
		t.Fatalf("expected viewport height to stay stable, got %d", metrics.ViewportHeight)
	}
}

func TestBuildInternalCaptureSegmentsUsesViewportSizedSlices(t *testing.T) {
	segments := buildInternalCaptureSegments(screenshotPlan{
		ViewportHeight: 900,
		DocumentWidth:  1440,
		CapturedHeight: 2100,
	}, config.Config{
		FullPageTileHeight: 2400,
	})

	if len(segments) != 3 {
		t.Fatalf("expected 3 internal segments, got %d", len(segments))
	}
	if segments[0].Height != 900 || segments[1].Y != 900 || segments[2].Height != 300 {
		t.Fatalf("unexpected internal segments: %#v", segments)
	}
}

func TestDocumentScreenshotClipForTileUsesDocumentCoordinates(t *testing.T) {
	clip := documentScreenshotClipForTile(screenshotTilePlan{
		ID:     "segment-2",
		Y:      1800,
		Width:  1440,
		Height: 900,
	})

	if clip.X != 0 {
		t.Fatalf("expected x=0, got %v", clip.X)
	}
	if clip.Y != 1800 {
		t.Fatalf("expected document y=1800, got %v", clip.Y)
	}
	if clip.Width != 1440 || clip.Height != 900 {
		t.Fatalf("unexpected clip size: %#v", clip)
	}
}

func TestScrolledTileCaptureRequestUsesViewportCaptureMode(t *testing.T) {
	req := scrolledTileCaptureRequest(screenshotTilePlan{
		ID:     "segment-2",
		Y:      1800,
		Width:  1440,
		Height: 900,
	})

	if req.Format != proto.PageCaptureScreenshotFormatPng {
		t.Fatalf("expected png format, got %q", req.Format)
	}
	if req.FromSurface {
		t.Fatal("expected scrolled tile capture to use view mode, not surface mode")
	}
	if req.CaptureBeyondViewport {
		t.Fatal("expected scrolled tile capture to stay within the real viewport")
	}
	if req.Clip == nil || req.Clip.Y != 0 {
		t.Fatalf("expected viewport-relative clip, got %#v", req.Clip)
	}
	if req.Clip.Width != 1440 || req.Clip.Height != 900 {
		t.Fatalf("unexpected scrolled clip size: %#v", req.Clip)
	}
}

func TestWithinScrollToleranceAllowsTinyViewportDrift(t *testing.T) {
	if !withinScrollTolerance(1800, 1802) {
		t.Fatal("expected tiny drift to stay within tolerance")
	}
	if withinScrollTolerance(1800, 1804) {
		t.Fatal("expected larger drift to fall outside tolerance")
	}
}

func TestComposeCapturedTilesPNGStitchesTilesInOrder(t *testing.T) {
	redTile := mustPNGTile(t, 4, 2, color.NRGBA{R: 255, A: 255})
	greenTile := mustPNGTile(t, 4, 2, color.NRGBA{G: 255, A: 255})

	composed, err := composeCapturedTilesPNG(4, 4, []capturedScreenshotTile{
		{ID: "tile-0", Y: 0, Width: 4, Height: 2, Data: redTile},
		{ID: "tile-1", Y: 2, Width: 4, Height: 2, Data: greenTile},
	})
	if err != nil {
		t.Fatalf("expected composed png, got error: %v", err)
	}

	decoded, err := png.Decode(bytes.NewReader(composed))
	if err != nil {
		t.Fatalf("expected png decode, got error: %v", err)
	}

	if got := color.NRGBAModel.Convert(decoded.At(1, 1)).(color.NRGBA); got.R != 255 || got.G != 0 {
		t.Fatalf("expected top tile to stay red, got %#v", got)
	}
	if got := color.NRGBAModel.Convert(decoded.At(1, 3)).(color.NRGBA); got.G != 255 || got.R != 0 {
		t.Fatalf("expected bottom tile to stay green, got %#v", got)
	}
}

func TestFinalizeScrollableScreenshotReturnsSingleComposedTile(t *testing.T) {
	screenshot, warnings, err := finalizeScrollableScreenshot(screenshotPlan{
		Strategy:       "tiled",
		ViewportWidth:  1200,
		ViewportHeight: 900,
		DocumentWidth:  4,
		DocumentHeight: 4,
		CapturedHeight: 4,
	}, []capturedScreenshotTile{
		{ID: "tile-0", Y: 0, Width: 4, Height: 2, MimeType: "image/png", Data: mustPNGTile(t, 4, 2, color.NRGBA{R: 255, A: 255})},
		{ID: "tile-1", Y: 2, Width: 4, Height: 2, MimeType: "image/png", Data: mustPNGTile(t, 4, 2, color.NRGBA{G: 255, A: 255})},
	})
	if err != nil {
		t.Fatalf("expected composed screenshot, got error: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %#v", warnings)
	}
	if screenshot.Strategy != "single" {
		t.Fatalf("expected composed screenshot to be exposed as single, got %q", screenshot.Strategy)
	}
	if screenshot.MimeType != "image/png" {
		t.Fatalf("expected png mime, got %q", screenshot.MimeType)
	}
	if len(screenshot.Tiles) != 1 || screenshot.Tiles[0].Height != 4 {
		t.Fatalf("expected single composed tile, got %#v", screenshot.Tiles)
	}
}

func TestFinalizeScrollableScreenshotFallsBackToRawTilesOnComposeFailure(t *testing.T) {
	screenshot, warnings, err := finalizeScrollableScreenshot(screenshotPlan{
		Strategy:       "tiled",
		ViewportWidth:  1200,
		ViewportHeight: 900,
		DocumentWidth:  4,
		DocumentHeight: 4,
		CapturedHeight: 4,
	}, []capturedScreenshotTile{
		{ID: "tile-0", Y: 0, Width: 4, Height: 2, MimeType: "image/png", Data: []byte("not-a-png")},
	})
	if err != nil {
		t.Fatalf("expected fallback instead of error, got %v", err)
	}
	if screenshot.Strategy != "tiled" {
		t.Fatalf("expected tiled fallback, got %q", screenshot.Strategy)
	}
	if len(screenshot.Tiles) != 1 {
		t.Fatalf("expected raw tile fallback, got %#v", screenshot.Tiles)
	}
	foundWarning := false
	for _, warning := range warnings {
		if warning == "Screenshot composition failed; returning raw capture tiles." {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Fatalf("expected composition fallback warning, got %#v", warnings)
	}
}

func TestEncodeScreenshotBytesReturnsBase64(t *testing.T) {
	encoded := encodeScreenshotBytes([]byte{0x00, 0xff, 0x10, 0x42})

	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("expected valid base64, got error: %v", err)
	}
	if len(decoded) != 4 || decoded[1] != 0xff {
		t.Fatalf("unexpected decoded payload: %v", decoded)
	}
}

func mustPNGTile(t *testing.T, width, height int, fill color.NRGBA) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetNRGBA(x, y, fill)
		}
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		t.Fatalf("expected png encode, got %v", err)
	}
	return buffer.Bytes()
}

func TestSnapshotRawResourcesCreatesImmutableCopy(t *testing.T) {
	resources := map[string]*rawResource{
		"req-1": {
			RequestID:  "req-1",
			URL:        "https://example.com/hero.jpg",
			Type:       "image",
			MIMEType:   "image/jpeg",
			Bytes:      128000,
			StatusCode: 200,
		},
	}

	snapshot := snapshotRawResources(resources)
	resources["req-1"].Bytes = 4096
	resources["req-1"].URL = "https://example.com/changed.jpg"

	if snapshot[0].Bytes != 128000 {
		t.Fatalf("expected snapshot bytes to stay frozen, got %d", snapshot[0].Bytes)
	}
	if snapshot[0].URL != "https://example.com/hero.jpg" {
		t.Fatalf("expected snapshot URL to stay frozen, got %s", snapshot[0].URL)
	}
}
