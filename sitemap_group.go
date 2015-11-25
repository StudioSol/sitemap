package sitemap

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

type SitemapGroup struct {
	name        string
	folder      string
	group_count int
	urls        []URL
	url_channel chan URL
	done        chan bool
}

//Add a sitemap.URL to the group
func (s *SitemapGroup) Add(url URL) {
	s.url_channel <- url
}

//Clean Urls not yet added to the group
func (s *SitemapGroup) Clear() {
	s.urls = []URL{}
}

//Returns one sitemap.URLSet of Urls not yet added to the group
func (s *SitemapGroup) getURLSet() URLSet {
	return URLSet{URLs: s.urls}
}

func (s *SitemapGroup) getSitemapName() string {
	return s.name + "_" + strconv.Itoa(s.group_count) + ".xml.gz"
}

//Saves the sitemap from the sitemap.URLSet
func (s *SitemapGroup) Create(url_set URLSet) {
	var path string
	var remnant []URL

	xml, err := createSitemapXml(url_set)

	if err == ErrMaxFileSize {
		//splits into two sitemaps recursively
		newlimit := MAXURLSETSIZE / 2
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

	sitemap_name := s.getSitemapName()
	path = filepath.Join(s.folder, sitemap_name)

	err = saveXml(xml, path)
	if err != nil {
		log.Fatal("File not saved:", err)
	}

	savedSitemaps = append(savedSitemaps, sitemap_name)
	s.group_count++
	s.Clear()

	//append remnant urls if exists
	if len(remnant) > 0 {
		s.urls = append(s.urls, remnant...)
	}

}

// Starts to run the given list of Sitemap Groups concurrently.
func CloseGroups(groups ...*SitemapGroup) (done <-chan bool) {
	var wg sync.WaitGroup
	wg.Add(len(groups))

	ch := make(chan bool, 1)
	for _, group := range groups {
		go func(g *SitemapGroup) {
			<-g.Close()
			wg.Done()
		}(group)
	}
	go func() {
		wg.Wait()
		ch <- true
	}()
	return ch
}

//Mandatory operation, handle the rest of the url that has not been added to any sitemap and add.
//Furthermore performs cleaning of variables and closes the channel group
func (s *SitemapGroup) Close() <-chan bool {
	var closeDone = make(chan bool, 1)
	close(s.url_channel)

	go func() {
		<-s.done
		closeDone <- true
	}()

	return closeDone
}

//Initialize channel
func (s *SitemapGroup) Initialize() {
	s.done = make(chan bool, 1)

	s.url_channel = make(chan URL)
	for entry := range s.url_channel {
		s.urls = append(s.urls, entry)
		if len(s.urls) == MAXURLSETSIZE {
			s.Create(s.getURLSet())
		}
	}

	//remnant urls
	s.Create(s.getURLSet())
	s.Clear()

	s.done <- true
}

//Configure name and folder of group
func (s *SitemapGroup) Configure(name string, folder string) {
	s.name = strings.Replace(name, ".xml.gz", "", 1)
	s.group_count = 1
	_, err := ioutil.ReadDir(folder)
	if err != nil {
		err = os.MkdirAll(folder, 0655)
		if err != nil {
			log.Fatal("Dir not allowed - ", err)
		}
	}
	s.folder = folder
}
