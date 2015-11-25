//Generates sitemaps and index files based on the sitemaps.org protocol.
//facilitates the creation of sitemaps for large amounts of urls.
// For a full guide visit https://github.com/StudioSol/Sitemap
package sitemap

import (
	"io/ioutil"
	"log"
	"strconv"
	"strings"
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

//Represents an group of sitemaps
type SitemapGroup struct {
	name        string
	urls        []URL
	url_channel chan URL
	group_count int
	folder      string
}

//Add a sitemap.URL to the group
func (s *SitemapGroup) Add(url URL) {
	s.url_channel <- url
}

//Mandatory operation, handle the rest of the url that has not been added to any sitemap and add.
//Furthermore performs cleaning of variables and closes the channel group
func (s *SitemapGroup) CloseGroup() {
	s.Create(s.getURLSet())
	close(s.url_channel)
	s.Clear()
}

//Clean Urls not yet added to the group
func (s *SitemapGroup) Clear() {
	s.urls = []URL{}
}

//Returns one sitemap.URLSet of Urls not yet added to the group
func (s *SitemapGroup) getURLSet() URLSet {
	return URLSet{URLs: s.urls}
}

//Saves the sitemap from the sitemap.URLSet
func (s *SitemapGroup) Create(url_set URLSet) {
	var remnant []URL
	xml, err := createSitemapXml(url_set)

	if err == ErrMaxFileSize {
		//splits into two sitemaps recursively
		newlimit := int(MAXURLSETSIZE) / 2
		s.Create(URLSet{URLs: url_set.URLs[newlimit:]})
		s.Create(URLSet{URLs: url_set.URLs[:newlimit]})
		return
	} else if err == ErrMaxUrlSetSize {
		remnant = url_set.URLs[MAXURLSETSIZE:]
		url_set.URLs = url_set.URLs[:MAXURLSETSIZE]
		xml, err = createSitemapXml(url_set)
	} else if err != nil {
		log.Fatal("File not saved:", err)
	}

	var sitemap_name string = s.name + "_" + strconv.Itoa(s.group_count) + ".xml.gz"
	var path string = s.folder + sitemap_name

	err = saveXml(xml, path)
	if err != nil {
		log.Fatal("File not saved:", err)
	}

	savedSitemaps = append(savedSitemaps, sitemap_name)
	log.Printf("Sitemap created on %s", path)

	s.group_count++
	s.Clear()

	//append remnant urls if exists
	s.urls = append(s.urls, remnant...)
}

//Creates a new group of sitemaps that used a common name.
//If the sitemap exceed the limit of 50k urls, new sitemaps will have a numeric suffix to the name. Example:
//- blog_1.xml.gz
//- blog_2.xml.gz
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
				s.Create(s.getURLSet())
			}
		}
	}()

	return s
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
