package main

import (
	"compress/flate"
	"compress/gzip"
	"encoding/json"
	"flag"
	"github.com/PuerkitoBio/goquery"
	crawler "github.com/david-wiles/crawl-project"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	_ "github.com/ClickHouse/clickhouse-go"
)

type metaTag struct {
	Key string `json:"key"`
	Val string `json:"val"`
}

type crawlResult struct {
	ReqHeaders map[string]string `json:"reqHeaders"`
	ResHeaders map[string]string `json:"resHeaders"`
	Method     string            `json:"method"`
	TS         time.Time         `json:"ts"`
	Status     int               `json:"status"`
	Title      string            `json:"title"`
	Heading1   []string          `json:"h1"`
	Heading2   []string          `json:"h2"`
	Heading3   []string          `json:"h3"`
	BodyText   string            `json:"bodyText"`
	MetaTags   [][]metaTag       `json:"metaTags"`
	JsonLd     []string          `json:"jsonLd"`
	ALinks     []string          `json:"aLinks"`
	Images     []string          `json:"images"`
	//ResSize      int               `json:"resSize"`
	JsResources  []string `json:"jsResources"`
	CssResources []string `json:"cssResources"`
	//Navigation   map[string]string `json:"navigation"`
}

func SplitListFiles(filename string) []string {
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		if !os.IsNotExist(err) {
			panic(err)
		}
		// Assign b to an empty byte array
		_, _ = os.Stdout.WriteString("File not found: " + filename)
		b = []byte{}
	}

	return strings.Split(string(b), "\n")
}

// Create a valid URL from an href
func GetURL(prev *url.URL, link string) string {
	if len(link) == 0 {
		return ""
	}

	if len(link) > 3 && link[:3] == "tel" {
		return ""
	}

	// Check if link is probably already a fully-formed URL
	if len(link) > 4 && link[:4] == "http" {
		return link
	}

	if len(link) > 6 && link[:6] == "mailto" {
		return ""
	}

	if link[0] != '/' {
		return prev.Scheme + "://" + path.Join(prev.Hostname(), prev.Path, link)
	}

	// Get the domain and create the proper url from the previous URL
	return prev.Scheme + "://" + path.Join(prev.Hostname(), link)
}

func main() {

	//var (
	//	startUrls        []string
	//	exclusionsRegexp []string
	//)

	// Start and excluded urls from files
	delayFlag := flag.String("delay", "", "Delay duration between identical domains")
	durationFlag := flag.String("duration", "", "Duration the crawler should run")
	//startUrlsFileFlag := flag.String("start", "", "Start urls")
	//exclusions := flag.String("excluded", "", "Excluded url regexp")
	//clickhouseFlag := flag.String("db", "", "Database connection string")

	flag.Parse()

	// Parse values from flags
	delay, err := time.ParseDuration(*delayFlag)
	if err != nil {
		panic(err)
	}

	dur, err := time.ParseDuration(*durationFlag)
	if err != nil {
		panic(err)
	}

	//startUrls = SplitListFiles(*startUrlsFileFlag)
	//exclusionsRegexp = SplitListFiles(*exclusions)

	//conn, err := sql.Open("clickhouse", *clickhouseFlag)
	//if err != nil {
	//	panic(err)
	//}
	//
	//// Test connection
	//if err := conn.Ping(); err != nil {
	//	panic(err)
	//}

	results := []crawlResult{}

	// Start crawler with config
	c := crawler.NewCrawler()
	c.Must(
		&crawler.QueueOption{crawler.NewQueue(65535)},
		&crawler.StartUrlsOption{[]string{
			"https://www.wku.edu",
		}},
		&crawler.RegexpURLOption{[]string{
			"https://www.wku.edu.*",
		}},
		&crawler.HeadersOption{map[string]string{
			"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
			"Upgrade-Insecure-Requests": "1",
			"Accept-Language":           "en-us",
			"Accept-Encoding":           "gzip, deflate",
			"User-Agent":                "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.0 Safari/605.1.15",
		}},
		&crawler.ResponseFuncOption{[]crawler.ResponseFunc{
			func(c *crawler.Crawler, resp *http.Response) bool {
				// write URL and status code to stdout
				_, _ = os.Stdout.WriteString(resp.Request.URL.String() + " " + resp.Status + "\n")
				return true
			},
			func(c *crawler.Crawler, resp *http.Response) bool {
				defer resp.Body.Close()

				var (
					reader io.ReadCloser
					err    error
				)
				switch resp.Header.Get("Content-Encoding") {
				case "gzip":
					reader, err = gzip.NewReader(resp.Body)
					if err != nil {
						c.Errors <- err
						return false
					}
				case "deflate":
					reader = flate.NewReader(resp.Body)
				default:
					reader = resp.Body
				}

				// Reject pages with HTML size greater than 20 mb
				doc, err := goquery.NewDocumentFromReader(io.LimitReader(reader, 20000000))
				if err != nil {
					c.Errors <- err
					return false
				}

				crawl := crawlResult{
					ReqHeaders: make(map[string]string),
					ResHeaders: make(map[string]string),
					MetaTags:   [][]metaTag{},
				}

				for k, v := range resp.Request.Header {
					crawl.ReqHeaders[k] = v[0]
				}

				for k, v := range resp.Header {
					crawl.ResHeaders[k] = v[0]
				}

				crawl.TS = time.Now()
				crawl.Method = resp.Request.Method
				crawl.Status = resp.StatusCode
				crawl.Title = doc.Find("title").Text()
				crawl.Heading1 = doc.Find("h1").Map(func(i int, s *goquery.Selection) string {
					return s.Text()
				})
				crawl.Heading2 = doc.Find("h2").Map(func(i int, s *goquery.Selection) string {
					return s.Text()
				})
				crawl.Heading3 = doc.Find("h3").Map(func(i int, s *goquery.Selection) string {
					return s.Text()
				})
				crawl.BodyText = doc.Find("body").Text()
				doc.Find("meta").Each(func(i int, s *goquery.Selection) {
					node := s.Get(0)
					meta := []metaTag{}
					for _, attr := range node.Attr {
						meta = append(meta, metaTag{attr.Key, attr.Val})
					}
					crawl.MetaTags = append(crawl.MetaTags, meta)
				})
				crawl.JsonLd = doc.Find("script[type='application/ld+json'").Map(func(i int, s *goquery.Selection) string {
					return s.Text()
				})
				crawl.Images = doc.Find("img").Map(func(i int, s *goquery.Selection) string {
					if src, ok := s.Attr("src"); ok {
						return src
					}
					return ""
				})
				crawl.JsResources = doc.Find("script").Map(func(i int, s *goquery.Selection) string {
					if src, ok := s.Attr("src"); ok {
						return src
					}
					return ""
				})
				crawl.CssResources = doc.Find("link[rel='stylesheet']").Map(func(i int, s *goquery.Selection) string {
					if href, ok := s.Attr("href"); ok {
						return href
					}
					return ""
				})

				// Add links to queue
				doc.Find("a").Each(func(i int, el *goquery.Selection) {
					if href, ok := el.Attr("href"); ok {
						u := GetURL(resp.Request.URL, href)
						if u != "" {
							crawl.ALinks = append(crawl.ALinks, u)
							c.Queue.Add(u)
						}
					}
				})

				// Insert all values into JSON map
				results = append(results, crawl)

				return true
			},
		}},
		&crawler.DelayOption{delay},
	)

	_ = time.AfterFunc(dur, func() {
		c.Abort()
	})
	<-c.Start()

	f, err := os.Create("output.json")
	if err != nil {
		panic(err)
	}
	encoder := json.NewEncoder(f)
	if err := encoder.Encode(results); err != nil {
		panic(err)
	}
}
