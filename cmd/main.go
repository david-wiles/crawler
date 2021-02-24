package main

import (
	"compress/flate"
	"compress/gzip"
	"database/sql"
	"flag"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	_ "github.com/ClickHouse/clickhouse-go"
	"github.com/PuerkitoBio/goquery"
	"github.com/david-wiles/crawl-project"
)

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

	var (
		//headers          = make(map[string]string)
		startUrls        []string
		exclusionsRegexp []string
	)

	// Start and excluded urls from files
	delayFlag := flag.String("delay", "", "Delay duration between identical domains")
	startUrlsFileFlag := flag.String("start", "", "Start urls")
	exclusions := flag.String("excluded", "", "Excluded url regexp")
	clickhouseFlag := flag.String("db", "", "Database connection string")

	flag.Parse()

	// Parse values from flags
	dur, err := time.ParseDuration(*delayFlag)
	if err != nil {
		panic(err)
	}
	startUrls = SplitListFiles(*startUrlsFileFlag)
	exclusionsRegexp = SplitListFiles(*exclusions)

	conn, err := sql.Open("clickhouse", *clickhouseFlag)
	if err != nil {
		panic(err)
	}

	// Test connection
	if err := conn.Ping(); err != nil {
		panic(err)
	}

	// Start crawler with config
	c := crawler.NewCrawler()
	c.Must(
		&crawler.QueueOption{crawler.NewQueue(65535)},
		&crawler.StartUrlsOption{startUrls},
		&crawler.RegexpExclusionsOption{exclusionsRegexp},
		&crawler.HeadersOption{map[string]string{
			"Accept":                    "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
			"Upgrade-Insecure-Requests": "1",
			"Accept-Language":           "en-us",
			"Accept-Encoding":           "gzip, deflate",
			"User-Agent":                "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.0 Safari/605.1.15",
		}},
		&crawler.ResponseFuncOption{[]crawler.ResponseFunc{
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

				doc, err := goquery.NewDocumentFromReader(io.LimitReader(reader, 20000000))
				if err != nil {
					c.Errors <- err
					return false
				}

				// Add links to queue
				doc.Find("a").Each(func(i int, el *goquery.Selection) {
					if href, ok := el.Attr("href"); ok {
						u := GetURL(resp.Request.URL, href)
						if u != "" {
							c.Queue.Add(u)
						}
					}
				})

				return true
			},
			func(c *crawler.Crawler, resp *http.Response) bool {
				// write URL and status code to stdout
				_, _ = os.Stdout.WriteString(resp.Request.URL.String() + " " + resp.Status + "\n")
				return true
			},
		}},
		&crawler.DelayOption{dur},
	)
	c.Start()
}
