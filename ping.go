package sitemap

import (
	"fmt"
	"log"
	"net/http"
	"sync"
)

//Sends a ping to search engines indicating that the index has been updated.
//Currently supports Google and Bing.
func PingSearchEngines(indexFile string) {
	var urls = []string{
		fmt.Sprintf("http://www.google.com/ping?sitemap=%s", indexFile),
		fmt.Sprintf("http://www.bing.com/ping?sitemap=%s", indexFile),
	}

	results := asyncHttpGets(urls)
	for result := range results {
		log.Printf("%s status: %s\n", result.url, result.response.Status)
	}
}

type HttpResponse struct {
	url      string
	response *http.Response
	err      error
}

func asyncHttpGets(urls []string) chan HttpResponse {
	ch := make(chan HttpResponse)
	go func() {
		var wg sync.WaitGroup
		for _, url := range urls {
			wg.Add(1)
			go func(url string) {
				resp, err := http.Get(url)
				if err != nil {
					log.Println("error", resp, err)
					wg.Done()
					return
				}
				resp.Body.Close()
				ch <- HttpResponse{url, resp, err}
				wg.Done()
			}(url)
		}
		wg.Wait()
		close(ch)
	}()
	return ch
}
