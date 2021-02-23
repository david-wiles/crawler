package crawler

// A queue interface just needs to be able to add, get, and close
type Queue interface {
	Add(string)
	Get() (string, bool)
	Close()
}

type DefaultQueue struct {
	queue  chan string
	store  []string
	isOpen bool
}

func NewQueue() *DefaultQueue {
	return &DefaultQueue{
		queue:  make(chan string, 65535),
		store:  []string{},
		isOpen: true,
	}
}

func (q *DefaultQueue) Add(u string) {
	if !q.isOpen {
		return
	}

	// Add the URL to the channel if it will accept strings
	// If it accepts the string, shift as many urls into the
	// channel as possible. Otherwise add to memory store
	select {
	case q.queue <- u:
		q.shiftElements()
	default:
		q.store = append(q.store, u)
	}
}

func (q *DefaultQueue) Get() (string, bool) {
	u, ok := <-q.queue
	if ok {
		go q.shiftElements()
		return u, true
	}
	return "", false
}

func (q *DefaultQueue) Close() {
	q.isOpen = false
	// Once the store is empty, we will close the queue
}

// Shift elements until the queue is full or the memory store is empty
func (q *DefaultQueue) shiftElements() {
	if len(q.store) > 0 {
		select {
		case q.queue <- q.store[0]:
			q.store = q.store[1:]
			q.shiftElements()
		default:
			return
		}
	} else if !q.isOpen {
		close(q.queue)
	}
}
