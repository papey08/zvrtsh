package app

import (
	"sync"
	"zvrtsh/internal/model"
)

// subscriptionBuffer is how many updates a subscriber may lag behind before
// it is dropped. A dropped subscriber has to subscribe again and reload the state.
const subscriptionBuffer = 64

// hub delivers updated sessions to the subscriptions of their players.
type hub struct {
	mu   sync.Mutex
	subs map[string]map[chan model.Session]struct{} // user name -> subscriptions
}

func newHub() *hub {
	return &hub{subs: make(map[string]map[chan model.Session]struct{})}
}

func (h *hub) subscribe(userName string) chan model.Session {
	h.mu.Lock()
	defer h.mu.Unlock()

	ch := make(chan model.Session, subscriptionBuffer)
	if h.subs[userName] == nil {
		h.subs[userName] = make(map[chan model.Session]struct{})
	}
	h.subs[userName][ch] = struct{}{}
	return ch
}

func (h *hub) unsubscribe(userName string, ch chan model.Session) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.subs[userName][ch]; ok {
		h.remove(userName, ch)
	}
}

// publish never blocks: a subscription with a full buffer is closed instead.
// The session must not be changed after it is published.
func (h *hub) publish(session model.Session) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, sheet := range session.Sheets {
		for ch := range h.subs[sheet.Owner] {
			select {
			case ch <- session:
			default:
				h.remove(sheet.Owner, ch)
			}
		}
	}
}

func (h *hub) remove(userName string, ch chan model.Session) {
	close(ch)
	delete(h.subs[userName], ch)
	if len(h.subs[userName]) == 0 {
		delete(h.subs, userName)
	}
}
