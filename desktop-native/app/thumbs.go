package app

import (
	"bytes"
	"context"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"net/url"

	"github.com/egoist/mygo/ui"
	"github.com/iferrnil/ai-news-radar/desktop-native/core"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// thumbs loads card thumbnails lazily: rows only build while visible (ui.List), and a row asks for its image
// as it builds. Bitmaps are downscaled to ≤ thumbW px wide and kept in a small LRU; failures fall back to no image.
const (
	thumbW      = 360
	thumbKeep   = 160
	thumbWorker = 4
)

type thumbState struct {
	bmp     *ui.Bitmap
	failed  bool
	pending bool
}

type thumbs struct {
	cache *core.ThumbCache
	m     map[string]*thumbState
	order []string
	sem   chan struct{}
}

func newThumbs(c *core.ThumbCache) *thumbs {
	return &thumbs{cache: c, m: map[string]*thumbState{}, sem: make(chan struct{}, thumbWorker)}
}

// get returns the bitmap if ready; otherwise starts a download (once) and returns nil.
func (a *App) thumb(u, referer string) *ui.Bitmap {
	t := a.thumbs
	if t == nil || t.cache == nil || u == "" {
		return nil
	}
	if s, ok := t.m[u]; ok {
		return s.bmp
	}
	s := &thumbState{pending: true}
	t.m[u] = s
	t.order = append(t.order, u)
	if len(t.order) > thumbKeep {
		drop := t.order[0]
		t.order = t.order[1:]
		delete(t.m, drop)
	}
	go func() {
		t.sem <- struct{}{}
		defer func() { <-t.sem }()
		data, err := t.cache.Get(context.Background(), u, referer)
		var img image.Image
		if err == nil {
			img, err = decodeThumb(data)
		}
		a.host.Post(func() {
			s.pending = false
			if err != nil {
				s.failed = true
				return
			}
			s.bmp = ui.NewBitmap(img)
		})
	}()
	return nil
}

func decodeThumb(data []byte) (image.Image, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	b := img.Bounds()
	if b.Dx() <= thumbW {
		return img, nil
	}
	h := b.Dy() * thumbW / b.Dx()
	dst := image.NewRGBA(image.Rect(0, 0, thumbW, max(1, h)))
	draw.ApproxBiLinear.Scale(dst, dst.Bounds(), img, b, draw.Src, nil)
	return dst, nil
}

func refererFor(it *core.Item) string {
	if it.Homepage != "" {
		return it.Homepage
	}
	if u, err := url.Parse(it.URL); err == nil {
		return u.Scheme + "://" + u.Host + "/"
	}
	return ""
}
