package main

import (
	"flag"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

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
		delay time.Duration
		//headers         = make(map[string]string)
		//startUrls       []string
		//excludedDomains []string
	)

	flag.DurationVar(&delay, "delay", 0, "Delay between requests to identical domains")

	// Parse header flags

	// Start and excluded urls from files
	//startUrlsFileFlag := flag.String("start", "", "Start urls")
	//exclusions := flag.String("excluded", "", "Excluded url regexp")

	flag.Parse()

	// Parse start and excluded urls from files
	//startUrls = SplitListFiles(*startUrlsFileFlag)
	//excludedDomains = SplitListFiles(*exclusions)

	// Start crawler with config
	c := crawler.NewCrawler()
	c.Must(
		&crawler.StartUrlsOption{[]string{
			"https://www.wku.edu",
			"https://www.eku.edu",
			"https://www.nku.edu",
			"https://www.uky.edu",
		}},
		&crawler.RegexpURLOption{[]string{"https://.*"}},
		&crawler.ResponseFuncOption{[]crawler.ResponseFunc{
			func(c *crawler.Crawler, resp *http.Response) bool {
				// write URL and status code to stdout
				_, _ = os.Stdout.WriteString(resp.Request.URL.String() + " " + resp.Status + "\n")
				return true
			},
			func(c *crawler.Crawler, resp *http.Response) bool {
				defer resp.Body.Close()

				doc, err := goquery.NewDocumentFromReader(resp.Body)
				if err != nil {
					c.Errors <- err
					return false
				}

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
		}},
		&crawler.DelayOption{time.Second * 3},
	)
	c.Start()
}
