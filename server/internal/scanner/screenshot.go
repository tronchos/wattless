package scanner

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"log/slog"
	"math"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/tronchos/wattless/server/internal/config"
)

type documentMetrics struct {
	ViewportWidth  int
	ViewportHeight int
	DocumentWidth  int
	DocumentHeight int
}
type layoutDocumentMetrics struct {
	ContentWidth  int
	ContentHeight int
	LayoutWidth   int
	LayoutHeight  int
}
type domDocumentMetrics struct {
	DocumentWidth  int `json:"document_width"`
	DocumentHeight int `json:"document_height"`
}
type screenshotTilePlan struct {
	ID     string
	Y      int
	Width  int
	Height int
}
type screenshotPlan struct {
	Strategy       string
	ViewportWidth  int
	ViewportHeight int
	DocumentWidth  int
	DocumentHeight int
	CapturedHeight int
	Tiles          []screenshotTilePlan
	Truncated      bool
}
type capturedScreenshotTile struct {
	ID       string
	Y        int
	Width    int
	Height   int
	MimeType string
	Data     []byte
}

func primeScrollableContent(ctx context.Context, page *rod.Page, metrics documentMetrics, cfg config.Config) ([]string, error) {
	if metrics.DocumentHeight <= metrics.ViewportHeight {
		return nil, nil
	}

	step := maxInt(int(math.Round(float64(metrics.ViewportHeight)*0.75)), 1)
	maxHeight := minInt(metrics.DocumentHeight, cfg.FullPageMaxHeight)
	deadline := time.Now().Add(cfg.FullPagePrimeMaxDuration)
	partial := false

	for y := step; y < maxHeight; y += step {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if time.Now().After(deadline) {
			partial = true
			break
		}

		if err := scrollTo(page, y); err != nil {
			return nil, err
		}
		if err := sleepWithContext(ctx, 150*time.Millisecond); err != nil {
			return nil, err
		}

		refreshed, err := measureDocument(page, cfg)
		if err == nil {
			metrics = refreshed
			maxHeight = minInt(metrics.DocumentHeight, cfg.FullPageMaxHeight)
		}
	}

	if err := scrollToTop(page); err != nil {
		return nil, err
	}
	if err := sleepWithContext(ctx, 300*time.Millisecond); err != nil {
		return nil, err
	}

	if partial {
		return []string{"Lazy content was partially hydrated before capture."}, nil
	}
	return nil, nil
}
func buildScreenshotPlan(metrics documentMetrics, cfg config.Config) screenshotPlan {
	capturedHeight := minInt(metrics.DocumentHeight, cfg.FullPageMaxHeight)
	if capturedHeight <= 0 {
		capturedHeight = metrics.ViewportHeight
	}

	strategy := "single"
	tileHeight := capturedHeight
	if capturedHeight > cfg.FullPageSingleShotThreshold {
		strategy = "tiled"
		tileHeight = maxInt(cfg.FullPageTileHeight, 1)
	}

	tiles := make([]screenshotTilePlan, 0, maxInt(1, int(math.Ceil(float64(capturedHeight)/float64(maxInt(tileHeight, 1))))))
	for y := 0; y < capturedHeight; y += tileHeight {
		height := minInt(tileHeight, capturedHeight-y)
		tiles = append(tiles, screenshotTilePlan{
			ID:     fmt.Sprintf("tile-%d", len(tiles)),
			Y:      y,
			Width:  metrics.DocumentWidth,
			Height: height,
		})
	}

	return screenshotPlan{
		Strategy:       strategy,
		ViewportWidth:  metrics.ViewportWidth,
		ViewportHeight: metrics.ViewportHeight,
		DocumentWidth:  metrics.DocumentWidth,
		DocumentHeight: metrics.DocumentHeight,
		CapturedHeight: capturedHeight,
		Tiles:          tiles,
		Truncated:      metrics.DocumentHeight > capturedHeight,
	}
}
func captureDocumentScreenshot(ctx context.Context, page *rod.Page, plan screenshotPlan, quality int, cfg config.Config) (Screenshot, []string, error) {
	if plan.Strategy != "tiled" {
		tiles := make([]ScreenshotTile, 0, len(plan.Tiles))
		for _, tilePlan := range plan.Tiles {
			tile, err := captureScreenshotTile(page, tilePlan, quality)
			if err != nil {
				return Screenshot{}, nil, err
			}
			tiles = append(tiles, tile)
		}

		return Screenshot{
			MimeType:       "image/webp",
			Strategy:       plan.Strategy,
			ViewportWidth:  plan.ViewportWidth,
			ViewportHeight: plan.ViewportHeight,
			DocumentWidth:  plan.DocumentWidth,
			DocumentHeight: plan.DocumentHeight,
			CapturedHeight: plan.CapturedHeight,
			Tiles:          tiles,
		}, nil, nil
	}

	return captureComposedDocumentScreenshot(ctx, page, plan, cfg)
}
func captureComposedDocumentScreenshot(ctx context.Context, page *rod.Page, plan screenshotPlan, cfg config.Config) (Screenshot, []string, error) {
	var warnings []string

	neutralized, neutralizeRaw, err := neutralizeScrollHijack(page)
	slog.Info("scroll_hijack_neutralization", "neutralized", neutralized, "error", err, "raw", neutralizeRaw)
	if err != nil {
		warnings = append(warnings, "Scroll hijack neutralization failed; capture may be incomplete.")
	} else if neutralized {
		defer func() { _ = restoreScrollHijack(page) }()

		refreshed, err := measureDocument(page, cfg)
		if err == nil {
			plan = buildScreenshotPlan(refreshed, cfg)
			slog.Info("scroll_hijack_remeasured", "document_height", plan.DocumentHeight, "captured_height", plan.CapturedHeight, "strategy", plan.Strategy)
		}

		if err := scrollToTop(page); err != nil {
			warnings = append(warnings, "Could not reset scroll after neutralization.")
		}

		screenshot, expandErr := captureWithScrollPrime(ctx, page, plan, cfg)
		slog.Info("scroll_prime_capture", "success", expandErr == nil, "error", expandErr, "tiles", len(screenshot.Tiles))
		if expandErr == nil {
			return screenshot, warnings, nil
		}
		warnings = append(warnings, fmt.Sprintf("Scroll prime capture failed: %v; falling back to tiled capture.", expandErr))
	}

	segments := buildInternalCaptureSegments(plan, cfg)
	rawTiles := make([]capturedScreenshotTile, 0, len(segments))
	for _, segment := range segments {
		if err := advanceViewportToYFast(ctx, page, segment.Y); err != nil {
			return Screenshot{}, nil, err
		}

		tile, err := captureViewportScreenshotTile(page, segment)
		if err != nil {
			return Screenshot{}, nil, err
		}
		rawTiles = append(rawTiles, tile)
	}

	_ = scrollToTop(page)

	screenshot, composeWarnings, err := finalizeScrollableScreenshot(plan, rawTiles)
	warnings = append(warnings, composeWarnings...)
	return screenshot, warnings, err
}
func captureWithScrollPrime(ctx context.Context, page *rod.Page, plan screenshotPlan, cfg config.Config) (Screenshot, error) {
	_, _ = page.Evaluate(rod.Eval(`() => {
		window.__wattlessOriginalRAF = window.requestAnimationFrame;
		window.requestAnimationFrame = () => 0;
	}`))
	defer func() {
		_, _ = page.Evaluate(rod.Eval(`() => {
			if (window.__wattlessOriginalRAF) {
				window.requestAnimationFrame = window.__wattlessOriginalRAF;
				delete window.__wattlessOriginalRAF;
			}
		}`))
	}()

	quality := minInt(cfg.FullPageCaptureQuality, 55)
	segments := buildInternalCaptureSegments(plan, cfg)
	tiles := make([]ScreenshotTile, 0, len(segments))

	for i, seg := range segments {
		if ctx.Err() != nil {
			return Screenshot{}, ctx.Err()
		}

		_, _ = page.Evaluate(rod.Eval(`y => {
			const root = document.scrollingElement || document.documentElement;
			root.scrollTop = y;
			if (document.body) document.body.scrollTop = y;
			window.scrollTo(0, y);
		}`, seg.Y))

		if err := sleepWithContext(ctx, 50*time.Millisecond); err != nil {
			return Screenshot{}, err
		}

		req := proto.PageCaptureScreenshot{
			Format:  proto.PageCaptureScreenshotFormatWebp,
			Quality: intPtr(quality),
			Clip: &proto.PageViewport{
				X:      0,
				Y:      float64(seg.Y),
				Width:  float64(seg.Width),
				Height: float64(seg.Height),
				Scale:  1,
			},
			CaptureBeyondViewport: true,
		}
		result, err := req.Call(page)
		if err != nil {
			return Screenshot{}, fmt.Errorf("tile %d (y=%d): %w", i, seg.Y, err)
		}

		tiles = append(tiles, ScreenshotTile{
			ID:         seg.ID,
			Y:          seg.Y,
			Width:      seg.Width,
			Height:     seg.Height,
			DataBase64: encodeScreenshotBytes(result.Data),
		})
	}

	_, _ = page.Evaluate(rod.Eval(`() => {
		const root = document.scrollingElement || document.documentElement;
		root.scrollTop = 0;
		window.scrollTo(0, 0);
	}`))

	return Screenshot{
		MimeType:       "image/webp",
		Strategy:       "tiled",
		ViewportWidth:  plan.ViewportWidth,
		ViewportHeight: plan.ViewportHeight,
		DocumentWidth:  plan.DocumentWidth,
		DocumentHeight: plan.DocumentHeight,
		CapturedHeight: plan.CapturedHeight,
		Tiles:          tiles,
	}, nil
}
func buildInternalCaptureSegments(plan screenshotPlan, cfg config.Config) []screenshotTilePlan {
	segmentHeight := maxInt(cfg.FullPageTileHeight, 1)
	if segmentHeight > maxInt(plan.ViewportHeight, 1) {
		segmentHeight = maxInt(plan.ViewportHeight, 1)
	}
	segments := make([]screenshotTilePlan, 0, maxInt(1, int(math.Ceil(float64(plan.CapturedHeight)/float64(segmentHeight)))))
	for y := 0; y < plan.CapturedHeight; y += segmentHeight {
		height := minInt(segmentHeight, plan.CapturedHeight-y)
		segments = append(segments, screenshotTilePlan{
			ID:     fmt.Sprintf("segment-%d", len(segments)),
			Y:      y,
			Width:  plan.DocumentWidth,
			Height: height,
		})
	}
	return segments
}
func neutralizeScrollHijack(page *rod.Page) (bool, string, error) {
	result, err := page.Evaluate(rod.Eval(`() => {
		const html = document.documentElement;
		const body = document.body;
		if (!html || !body) {
			return JSON.stringify({ neutralized: false, reason: "no_dom" });
		}

		const cs = (el) => window.getComputedStyle(el);
		const htmlStyle = cs(html);
		const bodyStyle = cs(body);

		const bodyHasDeepContent = body.scrollHeight > body.clientHeight * 1.5;
		const htmlLocked = htmlStyle.overflow.includes("hidden") || htmlStyle.overflowY === "hidden";
		const bodyLocked = bodyStyle.overflow.includes("hidden") || bodyStyle.overflowY === "hidden";

		const diag = {
			bodyScrollHeight: body.scrollHeight,
			bodyClientHeight: body.clientHeight,
			htmlOverflow: htmlStyle.overflow,
			htmlOverflowY: htmlStyle.overflowY,
			bodyOverflow: bodyStyle.overflow,
			bodyOverflowY: bodyStyle.overflowY,
			bodyHasDeepContent,
			htmlLocked,
			bodyLocked,
		};

		if (!bodyHasDeepContent || (!htmlLocked && !bodyLocked)) {
			return JSON.stringify({ neutralized: false, reason: "no_hijack", ...diag });
		}

		window.__wattlessOriginalStyles = {
			htmlOverflow: html.style.overflow,
			htmlOverflowY: html.style.overflowY,
			htmlHeight: html.style.height,
			htmlMaxHeight: html.style.maxHeight,
			htmlPosition: html.style.position,
			bodyOverflow: body.style.overflow,
			bodyOverflowY: body.style.overflowY,
			bodyHeight: body.style.height,
			bodyMaxHeight: body.style.maxHeight,
			bodyPosition: body.style.position,
		};

		html.style.overflow = "visible";
		html.style.overflowY = "visible";
		html.style.height = "auto";
		html.style.maxHeight = "none";
		html.style.position = "static";
		body.style.overflow = "visible";
		body.style.overflowY = "visible";
		body.style.height = "auto";
		body.style.maxHeight = "none";
		body.style.position = "static";

		const wrapper = body.querySelector(
			"[data-lenis-content], [data-scroll-container], [data-scroll-section], .smooth-wrapper, .locomotive-scroll"
		);
		if (wrapper) {
			wrapper.style.transform = "none";
		}

		body.scrollTop = 0;
		html.scrollTop = 0;
		window.scrollTo(0, 0);

		delete window.__wattlessScrollTarget;

		return JSON.stringify({
			neutralized: true,
			bodyScrollHeight: body.scrollHeight,
			htmlScrollHeight: html.scrollHeight,
		});
	}`))
	if err != nil {
		return false, "", err
	}

	raw := result.Value.Str()
	var info struct {
		Neutralized bool `json:"neutralized"`
	}
	if err := json.Unmarshal([]byte(raw), &info); err != nil {
		return false, raw, err
	}
	return info.Neutralized, raw, nil
}
func restoreScrollHijack(page *rod.Page) error {
	_, err := page.Evaluate(rod.Eval(`() => {
		const orig = window.__wattlessOriginalStyles;
		if (!orig) return;
		const html = document.documentElement;
		const body = document.body;
		html.style.overflow = orig.htmlOverflow;
		html.style.overflowY = orig.htmlOverflowY;
		html.style.height = orig.htmlHeight;
		html.style.maxHeight = orig.htmlMaxHeight;
		html.style.position = orig.htmlPosition;
		body.style.overflow = orig.bodyOverflow;
		body.style.overflowY = orig.bodyOverflowY;
		body.style.height = orig.bodyHeight;
		body.style.maxHeight = orig.bodyMaxHeight;
		body.style.position = orig.bodyPosition;
		delete window.__wattlessOriginalStyles;
		delete window.__wattlessScrollTarget;
	}`))
	return err
}
func advanceViewportToY(ctx context.Context, page *rod.Page, targetY int) error {
	if err := scrollTo(page, targetY); err != nil {
		return err
	}
	if err := waitForScrollPosition(ctx, page, targetY, 1200*time.Millisecond); err != nil {
		return err
	}

	return waitForViewportSettle(ctx, page)
}
func advanceViewportToYFast(ctx context.Context, page *rod.Page, targetY int) error {
	if err := scrollTo(page, targetY); err != nil {
		return err
	}
	if err := sleepWithContext(ctx, 80*time.Millisecond); err != nil {
		return err
	}
	return waitForNextPaint(ctx, page)
}
func waitForViewportSettle(ctx context.Context, page *rod.Page) error {
	if err := sleepWithContext(ctx, 60*time.Millisecond); err != nil {
		return err
	}
	if err := page.WaitDOMStable(100*time.Millisecond, 0); err != nil {
		return err
	}
	if err := waitForNextPaint(ctx, page); err != nil {
		return err
	}
	if err := waitForVisibleImages(ctx, page, 150*time.Millisecond); err != nil {
		return err
	}
	return waitForNextPaint(ctx, page)
}
func waitForScrollPosition(ctx context.Context, page *rod.Page, targetY int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		scrollY, err := readScrollY(page)
		if err != nil {
			return err
		}
		if withinScrollTolerance(scrollY, targetY) {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("viewport did not reach target scroll position %d (current %d)", targetY, scrollY)
		}
		if err := sleepWithContext(ctx, 50*time.Millisecond); err != nil {
			return err
		}
	}
}
func withinScrollTolerance(currentY, targetY int) bool {
	return absInt(currentY-targetY) <= 2
}
func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
func waitForVisibleImages(ctx context.Context, page *rod.Page, timeout time.Duration) error {
	done := make(chan error, 1)
	go func() {
		_, err := page.Evaluate(rod.Eval(`timeoutMs => new Promise((resolve) => {
			const images = Array.from(document.images || []).filter((img) => {
				const rect = img.getBoundingClientRect();
				return rect.bottom > 0 && rect.top < window.innerHeight && rect.right > 0 && rect.left < window.innerWidth;
			});
			if (images.length === 0) {
				resolve(true);
				return;
			}

			let settled = false;
			const finish = () => {
				if (settled) return;
				settled = true;
				resolve(true);
			};

			const timer = setTimeout(finish, timeoutMs);
			const waiters = images.map((img) => {
				if (img.complete) {
					return Promise.resolve();
				}
				if (typeof img.decode === "function") {
					return img.decode().catch(() => undefined);
				}
				return new Promise((imageResolve) => {
					img.addEventListener("load", imageResolve, { once: true });
					img.addEventListener("error", imageResolve, { once: true });
				});
			});

			Promise.allSettled(waiters).finally(() => {
				clearTimeout(timer);
				finish();
			});
		})`, int(timeout.Milliseconds())))
		done <- err
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}
func captureViewportScreenshotTile(page *rod.Page, tile screenshotTilePlan) (capturedScreenshotTile, error) {
	req := scrolledTileCaptureRequest(tile)

	result, err := req.Call(page)
	if err != nil {
		return capturedScreenshotTile{}, err
	}

	return capturedScreenshotTile{
		ID:       tile.ID,
		Y:        tile.Y,
		Width:    tile.Width,
		Height:   tile.Height,
		MimeType: "image/png",
		Data:     result.Data,
	}, nil
}
func finalizeScrollableScreenshot(plan screenshotPlan, rawTiles []capturedScreenshotTile) (Screenshot, []string, error) {
	composed, err := composeCapturedTilesPNG(plan.DocumentWidth, plan.CapturedHeight, rawTiles)
	if err == nil {
		return Screenshot{
			MimeType:       "image/png",
			Strategy:       "single",
			ViewportWidth:  plan.ViewportWidth,
			ViewportHeight: plan.ViewportHeight,
			DocumentWidth:  plan.DocumentWidth,
			DocumentHeight: plan.DocumentHeight,
			CapturedHeight: plan.CapturedHeight,
			Tiles: []ScreenshotTile{
				{
					ID:         "tile-0",
					Y:          0,
					Width:      plan.DocumentWidth,
					Height:     plan.CapturedHeight,
					DataBase64: encodeScreenshotBytes(composed),
				},
			},
		}, nil, nil
	}

	tiles := make([]ScreenshotTile, 0, len(rawTiles))
	for _, tile := range rawTiles {
		tiles = append(tiles, ScreenshotTile{
			ID:         tile.ID,
			Y:          tile.Y,
			Width:      tile.Width,
			Height:     tile.Height,
			DataBase64: encodeScreenshotBytes(tile.Data),
		})
	}

	return Screenshot{
		MimeType:       "image/png",
		Strategy:       "tiled",
		ViewportWidth:  plan.ViewportWidth,
		ViewportHeight: plan.ViewportHeight,
		DocumentWidth:  plan.DocumentWidth,
		DocumentHeight: plan.DocumentHeight,
		CapturedHeight: plan.CapturedHeight,
		Tiles:          tiles,
	}, []string{"Screenshot composition failed; returning raw capture tiles."}, nil
}
func composeCapturedTilesPNG(documentWidth, capturedHeight int, tiles []capturedScreenshotTile) ([]byte, error) {
	if documentWidth <= 0 || capturedHeight <= 0 {
		return nil, fmt.Errorf("invalid screenshot composition size %dx%d", documentWidth, capturedHeight)
	}
	if len(tiles) == 0 {
		return nil, fmt.Errorf("no screenshot tiles to compose")
	}

	canvas := image.NewNRGBA(image.Rect(0, 0, documentWidth, capturedHeight))
	for _, tile := range tiles {
		img, _, err := image.Decode(bytes.NewReader(tile.Data))
		if err != nil {
			return nil, fmt.Errorf("decode tile %s: %w", tile.ID, err)
		}
		bounds := img.Bounds()
		drawWidth := minInt(bounds.Dx(), documentWidth)
		drawHeight := minInt(bounds.Dy(), tile.Height)
		dstRect := image.Rect(0, tile.Y, drawWidth, minInt(tile.Y+drawHeight, capturedHeight))
		draw.Draw(canvas, dstRect, img, bounds.Min, draw.Src)
	}

	var buffer bytes.Buffer
	if err := png.Encode(&buffer, canvas); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}
func captureScreenshotTile(page *rod.Page, tile screenshotTilePlan, quality int) (ScreenshotTile, error) {
	req := proto.PageCaptureScreenshot{
		Format:                proto.PageCaptureScreenshotFormatWebp,
		Quality:               intPtr(quality),
		Clip:                  documentScreenshotClipForTile(tile),
		CaptureBeyondViewport: true,
	}

	result, err := req.Call(page)
	if err != nil {
		return ScreenshotTile{}, err
	}

	return ScreenshotTile{
		ID:         tile.ID,
		Y:          tile.Y,
		Width:      tile.Width,
		Height:     tile.Height,
		DataBase64: encodeScreenshotBytes(result.Data),
	}, nil
}
func documentScreenshotClipForTile(tile screenshotTilePlan) *proto.PageViewport {
	return &proto.PageViewport{
		X:      0,
		Y:      float64(tile.Y),
		Width:  float64(tile.Width),
		Height: float64(tile.Height),
		Scale:  1,
	}
}
func scrolledTileCaptureRequest(tile screenshotTilePlan) proto.PageCaptureScreenshot {
	return proto.PageCaptureScreenshot{
		Format:                proto.PageCaptureScreenshotFormatPng,
		Clip:                  viewportScreenshotClipForTile(tile),
		FromSurface:           false,
		CaptureBeyondViewport: false,
	}
}
func viewportScreenshotClipForTile(tile screenshotTilePlan) *proto.PageViewport {
	return &proto.PageViewport{
		X:      0,
		Y:      0,
		Width:  float64(tile.Width),
		Height: float64(tile.Height),
		Scale:  1,
	}
}
func scrollToTop(page *rod.Page) error {
	return scrollTo(page, 0)
}
func scrollToTopAndWait(ctx context.Context, page *rod.Page) (string, error) {
	if err := scrollToTop(page); err != nil {
		return "", err
	}

	deadline := time.Now().Add(1200 * time.Millisecond)
	for {
		scrollY, err := readScrollY(page)
		if err != nil {
			return "", err
		}
		if scrollY == 0 {
			if err := waitForNextPaint(ctx, page); err != nil {
				return "", err
			}
			settledScrollY, err := readScrollY(page)
			if err != nil {
				return "", err
			}
			if settledScrollY == 0 {
				return "", nil
			}
		}

		if time.Now().After(deadline) {
			return "Visual inspector snapshot could not fully return to the first viewport; fold and visibility hints may be less precise.", nil
		}
		if err := sleepWithContext(ctx, 50*time.Millisecond); err != nil {
			return "", err
		}
		if err := scrollToTop(page); err != nil {
			return "", err
		}
	}
}
func scrollTo(page *rod.Page, y int) error {
	_, err := page.Evaluate(rod.Eval(`targetY => {
		const clamp = (value, min, max) => Math.min(max, Math.max(min, value));
		const getCachedScroller = () => {
			const cached = window.__wattlessScrollTarget;
			if (!cached || !(cached instanceof HTMLElement) || !cached.isConnected) {
				return null;
			}
			const range = Math.max((cached.scrollHeight || 0) - (cached.clientHeight || 0), 0);
			return range > 0 ? cached : null;
		};
		const pickScroller = () => {
			const cached = getCachedScroller();
			if (cached) return cached;
			const root = document.scrollingElement || document.documentElement || document.body;
			let best = root;
			let bestRange = root ? Math.max((root.scrollHeight || 0) - (root.clientHeight || 0), 0) : 0;
			for (const el of document.querySelectorAll("*")) {
				if (!(el instanceof HTMLElement)) continue;
				const style = window.getComputedStyle(el);
				const overflowY = style.overflowY || "";
				if (!/(auto|scroll|overlay)/.test(overflowY)) continue;
				const range = Math.max((el.scrollHeight || 0) - (el.clientHeight || 0), 0);
				if (range <= 0) continue;
				if (range > bestRange + 32) {
					best = el;
					bestRange = range;
				}
			}
			window.__wattlessScrollTarget = best || root || null;
			return window.__wattlessScrollTarget;
		};

		const scroller = pickScroller();
		if (!scroller) {
			window.scrollTo(0, targetY);
			return Math.round(window.scrollY || document.documentElement.scrollTop || 0);
		}

		const maxY = Math.max((scroller.scrollHeight || 0) - (scroller.clientHeight || 0), 0);
		const nextY = clamp(targetY, 0, maxY);
		if (typeof scroller.scrollTo === "function") {
			scroller.scrollTo(0, nextY);
		} else {
			scroller.scrollTop = nextY;
		}
		return Math.round(scroller.scrollTop || window.scrollY || document.documentElement.scrollTop || 0);
	}`, y))
	return err
}
func readScrollY(page *rod.Page) (int, error) {
	result, err := page.Evaluate(rod.Eval(`() => {
		const cached = window.__wattlessScrollTarget;
		const best = cached && cached instanceof HTMLElement && cached.isConnected ? cached : (document.scrollingElement || document.documentElement || document.body);
		return Math.round((best && best.scrollTop) || window.scrollY || document.documentElement.scrollTop || 0);
	}`))
	if err != nil {
		return 0, err
	}
	return result.Value.Int(), nil
}
func waitForNextPaint(ctx context.Context, page *rod.Page) error {
	done := make(chan error, 1)
	go func() {
		_, err := page.Evaluate(rod.Eval(`() => new Promise((resolve) => {
			requestAnimationFrame(() => {
				requestAnimationFrame(() => resolve(true));
			});
		})`))
		done <- err
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}
func sleepWithContext(ctx context.Context, wait time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(wait):
		return nil
	}
}
func intPtr(value int) *int {
	return &value
}
func encodeScreenshotBytes(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}
func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}
func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}
