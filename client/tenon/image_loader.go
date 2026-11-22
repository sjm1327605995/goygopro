package tenon

import (
	"image"
	"sync"

	appservice "github.com/sjm1327605995/goygopro/client/app"
	"github.com/sjm1327605995/tenon/pkg/ui"
)

type cardImageRequest struct {
	loading bool
	waiters []func(image.Image)
}

var cardImageRequests = struct {
	sync.Mutex
	items map[uint32]*cardImageRequest
}{items: make(map[uint32]*cardImageRequest)}

var cardImageSlots = make(chan struct{}, 4)

var decodeCardImageForUI = func(service *appservice.App, code uint32) image.Image {
	return decodeCardImage(service, code)
}

func cachedCardImage(code uint32) image.Image {
	if cached, ok := decodedCardImages.Load(code); ok {
		return cached.(image.Image)
	}
	return nil
}

func requestCardImage(service *appservice.App, code uint32, callback func(image.Image)) {
	if callback == nil {
		return
	}
	if img := cachedCardImage(code); img != nil {
		callback(img)
		return
	}
	cardImageRequests.Lock()
	request := cardImageRequests.items[code]
	if request != nil && request.loading {
		request.waiters = append(request.waiters, callback)
		cardImageRequests.Unlock()
		return
	}
	request = &cardImageRequest{loading: true, waiters: []func(image.Image){callback}}
	cardImageRequests.items[code] = request
	cardImageRequests.Unlock()

	go func() {
		cardImageSlots <- struct{}{}
		img := decodeCardImageForUI(service, code)
		<-cardImageSlots
		ui.Post(func() {
			cardImageRequests.Lock()
			waiters := request.waiters
			if img == nil {
				delete(cardImageRequests.items, code)
			} else {
				request.loading = false
				request.waiters = nil
			}
			cardImageRequests.Unlock()
			for _, waiter := range waiters {
				waiter(img)
			}
		})
	}()
}
