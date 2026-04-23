package imagegrid

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg"
	"image/png"
	_ "image/png"
	"io"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"gthanks/internal/domain"
	"gthanks/internal/port"
)

const (
	defaultPerRow  = 5
	defaultWidth   = 1200
	defaultLimit   = 50
	defaultPadding = 0
	defaultSpace   = 0
	minPerRow      = 1
	maxPerRow      = 20
	minWidth       = 100
	maxWidth       = 4000
	minLimit       = 1
	maxLimit       = 10000
	minSpacing     = 0
	maxSpacing     = 500
	avatarWorkers  = 4
	avatarTTL      = 24 * time.Hour
)

type Shape string

const (
	ShapeCircle Shape = "circle"
	ShapeSquare Shape = "square"
)

type Options struct {
	PerRow  int
	Width   int
	Shape   Shape
	Limit   int
	Padding int
	Space   int
}

type Renderer struct {
	client *http.Client
	store  port.Store
}

func NewRenderer(store port.Store) *Renderer {
	return &Renderer{
		client: &http.Client{Timeout: 15 * time.Second},
		store:  store,
	}
}

func (r *Renderer) Render(ctx context.Context, summary []domain.SummaryItem, options Options) (image.Image, error) {
	options, err := normalizeOptions(options)
	if err != nil {
		return nil, err
	}

	summary = clampSummary(summary, options.Limit)
	if len(summary) == 0 {
		return nil, fmt.Errorf("no contributors with avatar images")
	}

	cols := min(options.PerRow, len(summary))
	cellSize := max(1, (options.Width-(options.Padding*2)-((cols-1)*options.Space))/cols)
	rows := int(math.Ceil(float64(len(summary)) / float64(cols)))
	height := (options.Padding * 2) + (rows * cellSize) + ((rows - 1) * options.Space)

	canvas := image.NewRGBA(image.Rect(0, 0, options.Width, height))
	avatars := r.fetchAvatars(ctx, summary)

	for index := range summary {
		avatar := avatars[index]
		if avatar == nil {
			continue
		}
		col := index % cols
		row := index / cols
		x := options.Padding + col*(cellSize+options.Space)
		y := options.Padding + row*(cellSize+options.Space)
		drawAvatar(canvas, avatar, image.Rect(x, y, x+cellSize, y+cellSize), options.Shape)
	}

	return canvas, nil
}

func EncodePNG(img image.Image, w http.ResponseWriter) error {
	return png.Encode(w, img)
}

func EncodePNGBytes(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func normalizeOptions(options Options) (Options, error) {
	if options.PerRow == 0 {
		options.PerRow = defaultPerRow
	}
	if options.Width == 0 {
		options.Width = defaultWidth
	}
	if options.Limit == 0 {
		options.Limit = defaultLimit
	}
	if options.Padding == 0 {
		options.Padding = defaultPadding
	}
	if options.Space == 0 {
		options.Space = defaultSpace
	}
	if options.Shape == "" {
		options.Shape = ShapeCircle
	}
	if options.PerRow < minPerRow || options.PerRow > maxPerRow {
		return Options{}, fmt.Errorf("per_row must be between %d and %d", minPerRow, maxPerRow)
	}
	if options.Width < minWidth || options.Width > maxWidth {
		return Options{}, fmt.Errorf("width must be between %d and %d", minWidth, maxWidth)
	}
	if options.Limit < minLimit || options.Limit > maxLimit {
		return Options{}, fmt.Errorf("limit must be between %d and %d", minLimit, maxLimit)
	}
	if options.Padding < minSpacing || options.Padding > maxSpacing {
		return Options{}, fmt.Errorf("padding must be between %d and %d", minSpacing, maxSpacing)
	}
	if options.Space < minSpacing || options.Space > maxSpacing {
		return Options{}, fmt.Errorf("space must be between %d and %d", minSpacing, maxSpacing)
	}
	if options.Shape != ShapeCircle && options.Shape != ShapeSquare {
		return Options{}, fmt.Errorf("shape must be one of: %s, %s", ShapeCircle, ShapeSquare)
	}
	minRequiredWidth := (options.Padding * 2) + (min(options.PerRow, options.Limit) * 1)
	if options.Width <= (options.Padding*2)+((options.PerRow-1)*options.Space) {
		return Options{}, fmt.Errorf("width is too small for the requested padding/space/per_row combination")
	}
	if options.Width < minRequiredWidth {
		return Options{}, fmt.Errorf("width is too small for the requested padding")
	}
	return options, nil
}

func NormalizeOptionsForCache(options Options) (Options, error) {
	return normalizeOptions(options)
}

func clampSummary(summary []domain.SummaryItem, limit int) []domain.SummaryItem {
	filtered := make([]domain.SummaryItem, 0, len(summary))
	for _, item := range summary {
		if strings.TrimSpace(item.AvatarURL) == "" {
			continue
		}
		filtered = append(filtered, item)
		if len(filtered) == limit {
			break
		}
	}
	return filtered
}

func (r *Renderer) fetchAvatars(ctx context.Context, summary []domain.SummaryItem) []image.Image {
	type job struct {
		index int
		url   string
	}

	avatars := make([]image.Image, len(summary))
	jobs := make(chan job)
	var wg sync.WaitGroup

	workers := min(avatarWorkers, len(summary))
	if workers == 0 {
		return avatars
	}

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range jobs {
				if ctx.Err() != nil {
					return
				}
				avatar, err := r.fetchAvatar(ctx, task.url)
				if err != nil {
					continue
				}
				avatars[task.index] = avatar
			}
		}()
	}

	for index, item := range summary {
		if ctx.Err() != nil {
			break
		}
		jobs <- job{index: index, url: item.AvatarURL}
	}
	close(jobs)
	wg.Wait()
	return avatars
}

func (r *Renderer) fetchAvatar(ctx context.Context, avatarURL string) (image.Image, error) {
	if r.store != nil {
		cached, err := r.store.GetAvatarCache(ctx, avatarURL)
		if err == nil && cached != nil && cached.ExpiresAt.After(time.Now().UTC()) {
			img, _, decodeErr := image.Decode(bytes.NewReader(cached.Content))
			if decodeErr == nil {
				return img, nil
			}
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, avatarURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("avatar request failed with status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if r.store != nil {
		_ = r.store.SaveAvatarCache(ctx, domain.AvatarCacheRecord{
			AvatarURL: avatarURL,
			Content:   body,
			ExpiresAt: time.Now().UTC().Add(avatarTTL),
		})
	}
	return img, nil
}

func drawAvatar(dst draw.Image, src image.Image, rect image.Rectangle, shape Shape) {
	resized := resizeToSquare(src, rect.Dx(), rect.Dy())
	for y := 0; y < rect.Dy(); y++ {
		for x := 0; x < rect.Dx(); x++ {
			if shape == ShapeCircle && !insideCircle(x, y, rect.Dx(), rect.Dy()) {
				continue
			}
			dst.Set(rect.Min.X+x, rect.Min.Y+y, resized.At(x, y))
		}
	}
}

func resizeToSquare(src image.Image, width, height int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	bounds := src.Bounds()
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			srcX := bounds.Min.X + (x*bounds.Dx())/width
			srcY := bounds.Min.Y + (y*bounds.Dy())/height
			dst.Set(x, y, src.At(srcX, srcY))
		}
	}
	return dst
}

func insideCircle(x, y, width, height int) bool {
	cx := float64(width-1) / 2
	cy := float64(height-1) / 2
	radius := math.Min(float64(width), float64(height)) / 2
	dx := float64(x) - cx
	dy := float64(y) - cy
	return (dx*dx + dy*dy) <= radius*radius
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
