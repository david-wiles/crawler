package crawler

import (
	"net/url"
	"regexp"
	"time"
)

type CrawlOption interface {
	SetOption(*Crawler) error
}

type StartUrlsOption struct {
	Urls []string
}

func (opt *StartUrlsOption) SetOption(c *Crawler) error {
	// Push all start URLs into Queue
	for _, u := range opt.Urls {
		// Ensure the URL is valid
		_, err := url.Parse(u)
		if err != nil {
			return err
		}

		c.Queue.Add(u)
	}

	return nil
}

type RegexpURLOption struct {
	Regexp []string
}

func (opt *RegexpURLOption) SetOption(c *Crawler) error {
	// Create map of regexps
	regexps := []*regexp.Regexp{}

	for _, exp := range opt.Regexp {
		re, err := regexp.Compile(exp)
		if err != nil {
			return err
		}

		regexps = append(regexps, re)
	}

	c.followRules = append(c.followRules, func(c *Crawler, href string) bool {
		// Search for the hostname in the excluded regexps, return true if not found
		u, err := url.Parse(href)
		if err != nil {
			// Can't follow the link if we wanted to
			return false
		}

		// Check url against all excluded regexps
		// If one matches, we should follow the link
		for _, re := range regexps {
			if re.MatchString(u.String()) {
				return true
			}
		}

		return false
	})
	return nil
}

type FollowFuncOption struct {
	FollowFuncs []FollowFunc
}

func (opt *FollowFuncOption) SetOption(c *Crawler) error {
	c.followRules = append(c.followRules, opt.FollowFuncs...)
	return nil
}

type ResponseFuncOption struct {
	ResponseFuncs []ResponseFunc
}

func (opt *ResponseFuncOption) SetOption(c *Crawler) error {
	c.responseRules = append(c.responseRules, opt.ResponseFuncs...)
	return nil
}

type DelayOption struct {
	Delay time.Duration
}

func (opt *DelayOption) SetOption(c *Crawler) error {
	c.domainMap = NewDomainMap(2048, opt.Delay)
	c.followRules = append(c.followRules, func(c *Crawler, u string) bool {
		// Sleep specified amount of time to ensure delay
		parsed, err := url.Parse(u)
		if err != nil {
			c.Errors <- err
			return false
		}

		prev := c.domainMap.Get(parsed.Hostname())
		now := time.Now()
		if prev != nil {
			// If this domain was requested in the past delay time, push it to the back
			// of the queue and return false so this worker can handle a different URL
			if prev.Add(opt.Delay).After(now) {
				c.Queue.Add(u)
				return false
			}
		}

		// Set the domain's last requested time to the current time stamp
		c.domainMap.Set(parsed.Hostname(), &now)
		return true
	})
	return nil
}

type WorkerCountOption struct {
	Count int
}

func (opt *WorkerCountOption) SetOption(c *Crawler) error {
	c.NumWorkers = opt.Count
	c.wPoll = make(chan bool, opt.Count)
	return nil
}
