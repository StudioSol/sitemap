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

type IndexGroup struct {
	name            string
	folder          string
	group_count     int
	sitemaps        []Sitemap
	sitemap_channel chan Sitemap
	done            chan bool
}

//Add a sitemap.Sitemap to the group
func (s *IndexGroup) Add(entry Sitemap) {
	s.sitemap_channel <- entry
}

//Clean Urls not yet added to the group
func (s *IndexGroup) Clear() {
	s.sitemaps = []Sitemap{}
}

//Returns one sitemap.Index of Urls not yet added to the group
func (s *IndexGroup) getSitemapSet() Index {
	return Index{Sitemaps: s.sitemaps}
}

func (s *IndexGroup) getSitemapName() string {
	return s.name + "_" + strconv.Itoa(s.group_count) + ".xml.gz"
}

//Saves the sitemap from the sitemap.URLSet
func (s *IndexGroup) Create(index Index) {
	var path string
	var remnant []Sitemap
	xml, err := createSitemapIndexXml(index)
	if err == ErrMaxFileSize {
		//splits into two sitemaps recursively
		newlimit := MAXURLSETSIZE / 2
		s.Create(Index{Sitemaps: index.Sitemaps[newlimit:]})
		s.Create(Index{Sitemaps: index.Sitemaps[:newlimit]})
		return
	} else if err == ErrMaxUrlSetSize {
		remnant = index.Sitemaps[MAXURLSETSIZE:]
		index.Sitemaps = index.Sitemaps[:MAXURLSETSIZE]
		xml, err = createSitemapIndexXml(index)
	}

	if err != nil {
		log.Fatal("File not saved:", err)
	}

	sitemap_name := s.getSitemapName()
	path = filepath.Join(s.folder, sitemap_name)

	err = saveXml(xml, path)
	if err != nil {
		log.Fatal("File not saved:", err)
	}
	s.group_count++
	s.Clear()
	//append remnant urls if exists
	if len(remnant) > 0 {
		s.sitemaps = append(s.sitemaps, remnant...)
	}
	log.Printf("Sitemap created on %s", path)

}

// Starts to run the given list of Sitemap Groups concurrently.
func CloseIndexGroups(groups ...*IndexGroup) (done <-chan bool) {
	var wg sync.WaitGroup
	wg.Add(len(groups))

	ch := make(chan bool, 1)
	for _, group := range groups {
		go func(g *IndexGroup) {
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
func (s *IndexGroup) Close() <-chan bool {
	var closeDone = make(chan bool, 1)
	close(s.sitemap_channel)

	go func() {
		<-s.done
		closeDone <- true
	}()

	return closeDone
}

//Initialize channel
func (s *IndexGroup) Initialize() {
	s.done = make(chan bool, 1)
	s.sitemap_channel = make(chan Sitemap)

	for entry := range s.sitemap_channel {
		s.sitemaps = append(s.sitemaps, entry)
		if len(s.sitemaps) == MAXURLSETSIZE {
			s.Create(s.getSitemapSet())
		}
	}

	//remnant urls
	s.Create(s.getSitemapSet())
	s.Clear()

	s.done <- true
}

//Configure name and folder of group
func (s *IndexGroup) Configure(name string, folder string) error {
	s.name = strings.Replace(name, ".xml.gz", "", 1)
	s.group_count = 1
	s.folder = folder
	_, err := ioutil.ReadDir(folder)
	if err != nil {
		err = os.MkdirAll(folder, 0655)
	}
	return err
}
