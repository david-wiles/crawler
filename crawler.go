package crawler

import (
	"net/http"
	"os"
	"runtime"
	"sync"
)

// A crawler is a very simple crawling engine
// The following and processing rules must be set at compile time,
// and the engine will use those rules during execution
type Crawler struct {

	// http client to use for all requests
	Client *http.Client

	// Maximum number of concurrent requests
	// This doesn't limit total number of goroutines, goroutines
	// should be limited using environment variables
	NumWorkers int

	// Errors occurring in goroutines
	Errors chan error

	// URL Queue
	Queue

	// Channel to notify that the crawler has finished its work
	Completed chan bool

	// Storage used to filter duplicates
	DuplicateFilter

	// Rules used by the engine to process urls and responses
	followRules   []FollowFunc
	responseRules []ResponseFunc

	rMu       *sync.Mutex
	rChan     chan bool
	nRequests int
	domainMap *DomainMap

	wPoll chan bool
}

// Get an initialized crawler engine
func NewCrawler() *Crawler {
	return &Crawler{
		Client:          http.DefaultClient,
		NumWorkers:      runtime.NumCPU(),
		Errors:          make(chan error),
		Queue:           NewQueue(),
		Completed:       make(chan bool),
		DuplicateFilter: &InMemoryDupFilter{},
		followRules:     []FollowFunc{},
		responseRules:   []ResponseFunc{},
		domainMap:       NewDomainMap(2048, 0),
		wPoll:           make(chan bool, runtime.NumCPU()),
	}
}

// Assign the options to the crawler or panic
func (c *Crawler) Must(opts ...CrawlOption) {
	for _, opt := range opts {
		if err := opt.SetOption(c); err != nil {
			panic(err)
		}
	}
}

// FollowFunc determines whether a request should be followed for the
// crawler. Each follow func assigned to the crawler will be evaluated, and
// the crawler will only follow the link if all are true
type FollowFunc func(*Crawler, string) bool

// ResponseFunc handles the HTTP response from a single request. Each function
// in the chain is evaluated as long as the previous one returns true
type ResponseFunc func(*Crawler, *http.Response) bool

// DownloadFunc's are called to handle the download of content after the initial
// response has been processed. Each DownloadFunc assigned to a crawler will be
// called, unless a download func in the chain returns false
type DownloadFunc func(*Crawler, *http.Response) bool

func (c *Crawler) Start() {
	// Fill worker poller with messages since all workers are available
	// Once a worker is finished with a request, it will send a message
	// indicating that the worker is ready to accept more work
	for i := 0; i < c.NumWorkers; i++ {
		c.wPoll <- true
	}

	go c.consumeErrors()

	// Consume all URLs in the queue
	// sendWork will block until a worker is ready to accept the url
	u, ok := c.Queue.Get()
	for ok {
		c.sendWork(u)
		u, ok = c.Queue.Get()
	}

	// Send message indicating that all URLs have finished processing.
	c.Completed <- true
}

// Abort a crawl by closing URL queue
// All processing and requests are also aborted
func (c *Crawler) Abort() {
	// Set processing rules to skip all requests and responses
	c.followRules = []FollowFunc{func(c *Crawler, u string) bool { return false }}
	c.responseRules = []ResponseFunc{func(c *Crawler, r *http.Response) bool { return false }}
	// Close URL queue to wind down requests
	c.Queue.Close()
}

// Returns the result of checking all follow rules for the url
// Returns false if the URL has already been visited
func (c *Crawler) shouldFollowURL(u string) bool {
	if c.DuplicateFilter.HasVisited(u) {
		return false
	}

	for _, fn := range c.followRules {
		if !fn(c, u) {
			return false
		}
	}

	return true
}

// Send the work to the first available worker
// When a worker is ready for a new URL, it polls for a new URL
func (c *Crawler) sendWork(u string) {
	if !c.shouldFollowURL(u) {
		return
	}
	<-c.wPoll
	go c.crawlURL(u)
}

func (c *Crawler) crawlURL(u string) {
	// Notify the main thread that the worker is ready to accept
	// work regardless of where the thread returns
	defer c.notifyReady()
	resp, err := c.doRequest(u)
	if err != nil {
		c.Errors <- err
		return
	}

	// Add URL to the duplicated URL filter
	c.DuplicateFilter.Visited(u)

	// Process response in separate goroutine
	go c.processResponse(resp)
}

func (c *Crawler) doRequest(u string) (*http.Response, error) {
	// Create and send HTTP request
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.0 Safari/605.1.15")
	return c.Client.Do(req)
}

// Indicate that this worker is ready to process another URL
func (c *Crawler) notifyReady() {
	c.wPoll <- true
}

func (c *Crawler) processResponse(resp *http.Response) {
	chain := true
	for _, fn := range c.responseRules {
		if chain {
			chain = fn(c, resp)
		}
	}
}

// Write all errors to stderr until channel is closed
func (c *Crawler) consumeErrors() {
	for err := range c.Errors {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
	}
}
