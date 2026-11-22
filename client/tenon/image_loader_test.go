package tenon

import (
	"image"
	"sync/atomic"
	"testing"
	"time"

	appservice "github.com/sjm1327605995/goygopro/client/app"
	"github.com/sjm1327605995/tenon/pkg/ui"
)

type loaderTestInput struct{}

func (loaderTestInput) Cursor() (float32, float32)           { return 0, 0 }
func (loaderTestInput) Wheel() (float32, float32)            { return 0, 0 }
func (loaderTestInput) MousePressed(ui.MouseButton) bool     { return false }
func (loaderTestInput) MouseJustPressed(ui.MouseButton) bool { return false }
func (loaderTestInput) KeyPressed(ui.InputKey) bool          { return false }
func (loaderTestInput) KeyJustPressed(ui.InputKey) bool      { return false }
func (loaderTestInput) TypedChars() []rune                   { return nil }

func TestCardImageRequestsShareInFlightDecode(t *testing.T) {
	const code = uint32(990001)
	decodedCardImages.Delete(code)
	cardImageRequests.Lock()
	delete(cardImageRequests.items, code)
	cardImageRequests.Unlock()
	originalDecoder := decodeCardImageForUI
	defer func() {
		decodeCardImageForUI = originalDecoder
		decodedCardImages.Delete(code)
		cardImageRequests.Lock()
		delete(cardImageRequests.items, code)
		cardImageRequests.Unlock()
	}()

	started := make(chan struct{}, 2)
	release := make(chan struct{})
	var decodes atomic.Int32
	decodeCardImageForUI = func(*appservice.App, uint32) image.Image {
		decodes.Add(1)
		started <- struct{}{}
		<-release
		img := image.NewRGBA(image.Rect(0, 0, 1, 1))
		decodedCardImages.Store(code, img)
		return img
	}
	runtime := ui.NewBackendRuntime(ui.Text("loader"), ui.WindowConfig{}, loaderTestInput{})
	defer runtime.Close()
	done := make(chan struct{}, 2)
	requestCardImage(appservice.NewApp(), code, func(image.Image) { done <- struct{}{} })
	requestCardImage(appservice.NewApp(), code, func(image.Image) { done <- struct{}{} })
	<-started
	if got := decodes.Load(); got != 1 {
		t.Fatalf("in-flight decodes = %d, want 1", got)
	}
	cardImageRequests.Lock()
	waiters := len(cardImageRequests.items[code].waiters)
	cardImageRequests.Unlock()
	if waiters != 2 {
		t.Fatalf("coalesced waiters = %d, want 2", waiters)
	}
	close(release)
	deadline := time.Now().Add(time.Second)
	for len(done) < 2 && time.Now().Before(deadline) {
		runtime.Frame(1)
		time.Sleep(time.Millisecond)
	}
	if len(done) != 2 {
		t.Fatal("coalesced image callbacks did not complete on the UI queue")
	}
	if cap(cardImageSlots) != 4 {
		t.Fatalf("image decoder concurrency = %d, want 4", cap(cardImageSlots))
	}
}
