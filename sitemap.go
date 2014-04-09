//Generates sitemaps and index files based on the sitemaps.org protocol.
//facilitates the creation of sitemaps for large amounts of urls.
// For a full guide visit https://github.com/StudioSol/Sitemap
package sitemap

import (
	"compress/gzip"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

var savedSitemaps []string

//clean array of already generated sitemaps (not delete files)
func ClearSavedSitemaps() {
	savedSitemaps = []string{}
}

//returns the url of already generated sitemaps
func GetSavedSitemaps() []string {
	return savedSitemaps
}

//Creates a new group of sitemaps that used a common name.
//If the sitemap exceed the limit of 50k urls, new sitemaps will have a numeric suffix to the name. Example:
//- blog_1.xml.gz
//- blog_2.xml.gz
func NewSitemapGroup(folder string, name string) *SitemapGroup {
	s := new(SitemapGroup)
	s.Configure(name, folder)
	go s.Initialize()
	return s
}

//Creates a new group of sitemaps indice that used a common name.
//If the sitemap exceed the limit of 50k urls, new sitemaps will have a numeric suffix to the name. Example:
//- blog_1.xml.gz
//- blog_2.xml.gz
func NewIndexGroup(folder string, name string) *IndexGroup {
	s := new(IndexGroup)
	s.Configure(name, folder)
	go s.Initialize()
	return s
}

//Save and gzip xml
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

//Search all the xml.gz sitemaps_dir directory, uses the modified date of the file as lastModified
//path_index is included for the function does not include the url of the index in your own content, if it is present in the same directory.
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

//Returns an index sitemap starting from a slice of urls
func CreateIndexBySlice(urls []string, public_url string) (index Index) {

	index = Index{Sitemaps: []Sitemap{}}

	if len(urls) > 0 {
		for _, fileName := range urls {
			lastModified := time.Now()
			index.Sitemaps = append(index.Sitemaps, Sitemap{Loc: public_url + fileName, LastMod: &lastModified})
		}
	}

	return
}

//Creates and gzip the xml index
func CreateSitemapIndex(indexFilePath string, index Index) (err error) {

	//create xml
	indexXml, err := createSitemapIndexXml(index)
	if err != nil {
		return err
	}
	err = saveXml(indexXml, indexFilePath)
	log.Printf("Sitemap Index created on %s", indexFilePath)
	return err
}

//Sends a ping to search engines indicating that the index has been updated.
//Currently supports Google and Bing.
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
