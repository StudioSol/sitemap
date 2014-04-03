package sitemap

import (
	"compress/gzip"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

var savedSitemaps []string

type SitemapGroup struct {
	name        string
	urls        []URL
	url_channel chan URL
	group_count int
	folder      string
}

type HttpResponse struct {
	url      string
	response *http.Response
	err      error
}

func (s *SitemapGroup) Add(url URL) {
	s.url_channel <- url
}

func (s *SitemapGroup) CloseGroup() {
	s.Create(s.getURLSet())
	close(s.url_channel)
	s.Clear()
}

func (s *SitemapGroup) Clear() {
	s.urls = []URL{}
}

func (s *SitemapGroup) getURLSet() URLSet {
	return URLSet{URLs: s.urls}
}

func (s *SitemapGroup) Create(url_set URLSet) {

	xml := createXML(url_set)
	var sitemap_name string = s.name + "_" + strconv.Itoa(s.group_count) + ".xml.gz"
	var path string = s.folder + sitemap_name

	err := saveXml(xml, path)

	if err != nil {
		log.Fatal("File not saved:", err)
	}
	savedSitemaps = append(savedSitemaps, sitemap_name)
	log.Printf("Sitemap created on %s", path)
	s.group_count++

}

func ClearSavedSitemaps() {
	savedSitemaps = []string{}
}
func GetSavedSitemaps() []string {
	return savedSitemaps
}

func NewSitemapGroup(folder string, name string) *SitemapGroup {
	s := new(SitemapGroup)
	s.name = strings.Replace(name, ".xml.gz", "", 1)
	s.group_count = 1
	s.url_channel = make(chan URL)
	_, err := ioutil.ReadDir(folder)
	if err != nil {
		log.Fatal("Dir not allowed - ", err)
	}
	s.folder = folder

	go func() {
		for entry := range s.url_channel {

			s.urls = append(s.urls, entry)

			if len(s.urls) == MAXURLSETSIZE {

				go func(urls URLSet) {
					s.Create(urls)
				}(s.getURLSet())

				s.Clear()
			}
		}
	}()

	return s
}

func createXML(group URLSet) (sitemapXml []byte) {
	sitemapXml, err := createSitemapXml(group)
	if err != nil {
		log.Fatal("work failed:", err)
	}
	return
}

func saveXml(xmlFile []byte, path string) (err error) {

	fo, err := os.Create(path)
	if err != nil {
		return err
	}
	defer fo.Close()

	zip := gzip.NewWriter(fo)
	defer zip.Close()
	_, err = zip.Write(xmlFile)
	if err != nil {
		return err
	}

	return err

}

func CreateIndexByScanDir(targetDir string, indexFileName string, public_url string) (index Index) {

	index = Index{Sitemaps: []Sitemap{}}

	fs, err := ioutil.ReadDir(targetDir)
	if err != nil {
		return
	}

	for _, f := range fs {
		if strings.HasSuffix(f.Name(), ".xml.gz") && !strings.HasSuffix(indexFileName, f.Name()) {
			lastModified := f.ModTime()
			index.Sitemaps = append(index.Sitemaps, Sitemap{Loc: public_url + f.Name(), LastMod: &lastModified})
		}
	}
	return
}

func CreateIndexBySlice(urls []string, public_url string) (index Index) {

	index = Index{Sitemaps: []Sitemap{}}

	if len(urls) > 0 {
		for _, fileName := range urls {
			index.Sitemaps = append(index.Sitemaps, Sitemap{Loc: public_url + fileName, LastMod: &time.Now()})
		}
	}

	return
}

func CreateSitemapIndex(indexFilePath string, index Index) (err error) {

	//create xml
	indexXml, err := createSitemapIndexXml(index)
	if err != nil {
		return err
	}
	//touch path
	fo, err := os.Create(indexFilePath)
	if err != nil {
		return err
	}
	defer fo.Close()
	//Save gzip
	zip := gzip.NewWriter(fo)
	defer zip.Close()
	_, err = zip.Write(indexXml)
	if err != nil {
		return err
	}

	log.Printf("Sitemap Index created on %s", indexFilePath)
	return err
}

func PingSearchEngines(indexFile string) {
	var urls = []string{
		"http://www.google.com/webmasters/tools/ping?sitemap=" + indexFile,
		"http://www.bing.com/ping?sitemap=" + indexFile,
	}

	results := asyncHttpGets(urls)

	for result := range results {
		log.Printf("%s status: %s\n", result.url, result.response.Status)
	}

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
