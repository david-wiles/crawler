package crawler

import (
	"sync"
	"time"
)

// A queue interface just needs to be able to add, get, and close
type Queue interface {
	Add(string)
	PushBack(string)
	Get() (string, bool)
	Close()
}

// DefaultQueue is an in-memory implementation of Queue
// This is used by default if a custom implementation is not
// specified by an Option
type DefaultQueue struct {

	// Indicates whether this queue will accept new URLs
	isOpen bool

	// This queue uses channels to send URLs to worker processes and
	// a slice to store URLs until they are needed.
	//
	//queue is the default channel used to send to workers
	//
	// urgentQueue is only used if
	// 1: a thread is blocked while waiting to get a URL
	// 2: another thread is simultaneously trying to send
	//
	// Normally, adding to the queue will put a URL in the back of the
	// in-memory slice. If a thread is waiting on a URL, it would be
	// deadlocked until a different thread called Get() and triggered
	// a flush of the slice to the channel
	queue       chan string
	urgentQueue chan string
	mu          *sync.Mutex
	memory      []string

	cycleCount int
	maxSize    int
}

func NewQueue(maxSize int) *DefaultQueue {
	return &DefaultQueue{
		isOpen:      true,
		queue:       make(chan string, maxSize),
		urgentQueue: make(chan string),
		mu:          &sync.Mutex{},
		memory:      []string{},
		cycleCount:  0,
		maxSize:     maxSize,
	}
}

// Add a new URL to the back of the queue
// If the queue is closed, the function will return and no URL will be added
// If a receiver is waiting on urgentQueue, the URL will go directly to the channel
// Otherwise, the URL will be added to memory
func (q *DefaultQueue) Add(u string) {
	if !q.isOpen {
		return
	}

	select {
	case q.urgentQueue <- u:
		// Send the string to urgent queue if a thread is waiting
	default:
		// Add the URL to memory
		q.addMemory(u)
	}
}

// Used to artificially add delay when adding URLs back to the queue
func (q *DefaultQueue) PushBack(u string) {
	<-time.After(100 * time.Millisecond)
	q.Add(u)
}

// Lock the memory before appending an element
func (q *DefaultQueue) addMemory(u string) {
	q.mu.Lock()

	// If the length of memory is equal to the maximum, ignore the input
	if len(q.memory) < q.maxSize {
		q.memory = append(q.memory, u)
	}
	q.mu.Unlock()
}

// Attempt to get an element from the channel. If the channel
// is empty, we should move as many elements as possible from
// memory to the channel and then send from the channel
func (q *DefaultQueue) Get() (u string, ok bool) {
	select {
	case u, ok = <-q.queue:
		return u, ok
	default:
		// If a value isn't ready, we will shift everything we can
		// from memory into queue and send the queue again
		q.shiftQueue()
	}

	// Get string from regular queue if it is available
	// Otherwise, receive from urgent queue once it has a value
	select {
	case u, ok = <-q.queue:
	default:
		u, ok = <-q.urgentQueue
	}
	return u, ok
}

func (q *DefaultQueue) Close() {
	q.isOpen = false
	// Once the memory is empty, we will close the queue
}

// Lock the queue's memory before shifting elements
func (q *DefaultQueue) shiftQueue() {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Skip function call if array is empty
	for len(q.memory) > 0 {
		select {
		case q.queue <- q.memory[0]:
			q.memory = q.memory[1:]
		default:
			// The channel is full, so we are done
			return
		}
	}

	// If we get here, the memory is empty so
	// we should check whether the queue is open
	if !q.isOpen {
		close(q.queue)
		close(q.urgentQueue)
	}
}
