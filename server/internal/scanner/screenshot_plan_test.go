package scanner

import (
	"encoding/base64"
	"testing"

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
