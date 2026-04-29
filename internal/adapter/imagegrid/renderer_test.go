package imagegrid

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"gthanks/internal/domain"
)

func TestRenderSquareGrid(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		img := image.NewRGBA(image.Rect(0, 0, 4, 4))
		for y := 0; y < 4; y++ {
			for x := 0; x < 4; x++ {
				img.Set(x, y, color.RGBA{255, 0, 0, 255})
			}
		}
		_ = png.Encode(w, img)
	}))
	defer server.Close()

	renderer := NewRenderer(nil)
	img, err := renderer.Render(context.Background(), []domain.SummaryItem{
		{Login: "alice", AvatarURL: server.URL},
		{Login: "bob", AvatarURL: server.URL},
	}, Options{PerRow: 2, Width: 200, Shape: ShapeSquare, Limit: 2, SpaceSet: true})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if img.Bounds().Dx() != 200 {
		t.Fatalf("expected width 200, got %d", img.Bounds().Dx())
	}
	if img.Bounds().Dy() != 100 {
		t.Fatalf("expected height 100 with zero padding/space, got %d", img.Bounds().Dy())
	}
}

func TestRenderCircleLeavesCornersBackground(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		img := image.NewRGBA(image.Rect(0, 0, 4, 4))
		for y := 0; y < 4; y++ {
			for x := 0; x < 4; x++ {
				img.Set(x, y, color.RGBA{0, 0, 255, 255})
			}
		}
		_ = png.Encode(w, img)
	}))
	defer server.Close()

	renderer := NewRenderer(nil)
	rendered, err := renderer.Render(context.Background(), []domain.SummaryItem{
		{Login: "alice", AvatarURL: server.URL},
	}, Options{PerRow: 1, Width: 200, Shape: ShapeCircle, Limit: 1})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	rgba := rendered.(*image.RGBA)
	got := color.RGBAModel.Convert(rgba.At(0, 0)).(color.RGBA)
	want := color.RGBA{0, 0, 0, 0}
	if got != want {
		t.Fatalf("expected transparent background at corner for circle crop, got %#v", got)
	}
}

func TestRenderRespectsPaddingAndSpace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		img := image.NewRGBA(image.Rect(0, 0, 4, 4))
		for y := 0; y < 4; y++ {
			for x := 0; x < 4; x++ {
				img.Set(x, y, color.RGBA{0, 255, 0, 255})
			}
		}
		_ = png.Encode(w, img)
	}))
	defer server.Close()

	renderer := NewRenderer(nil)
	img, err := renderer.Render(context.Background(), []domain.SummaryItem{
		{Login: "alice", AvatarURL: server.URL},
		{Login: "bob", AvatarURL: server.URL},
		{Login: "charlie", AvatarURL: server.URL},
	}, Options{PerRow: 2, Width: 200, Shape: ShapeSquare, Limit: 3, Padding: 10, Space: 5})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if img.Bounds().Dy() != 199 {
		t.Fatalf("expected height 199 with padding and space, got %d", img.Bounds().Dy())
	}
}

func TestNormalizeOptionsValidation(t *testing.T) {
	_, err := normalizeOptions(Options{Width: 99, Shape: ShapeSquare})
	if err == nil {
		t.Fatal("expected width validation error")
	}
}

func TestNormalizeOptionsDefaultsAndMaxLimit(t *testing.T) {
	defaulted, err := normalizeOptions(Options{})
	if err != nil {
		t.Fatalf("normalize default options: %v", err)
	}
	if defaulted.PerRow != 12 {
		t.Fatalf("expected default per_row 12, got %d", defaulted.PerRow)
	}
	if defaulted.Width != 1920 {
		t.Fatalf("expected default width 1920, got %d", defaulted.Width)
	}
	if defaulted.Limit != 1000 {
		t.Fatalf("expected default limit 1000, got %d", defaulted.Limit)
	}
	if defaulted.Padding != 0 {
		t.Fatalf("expected default padding 0, got %d", defaulted.Padding)
	}
	if defaulted.Space != 12 {
		t.Fatalf("expected default space 12, got %d", defaulted.Space)
	}

	maxed, err := normalizeOptions(Options{Limit: math.MaxInt})
	if err != nil {
		t.Fatalf("normalize max limit: %v", err)
	}
	if maxed.Limit != math.MaxInt {
		t.Fatalf("expected max limit %d, got %d", math.MaxInt, maxed.Limit)
	}
}
