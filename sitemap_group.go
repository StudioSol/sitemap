package sitemap

import (
	"io/ioutil"
	"log"
	"strconv"
	"strings"
)

type SitemapGroup struct {
	name        string
	folder      string
	group_count int
	urls        []URL
	url_channel chan URL
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

//Create sitemap XML from a URLSet
func (s *SitemapGroup) createXML(group URLSet) (sitemapXml []byte) {
	sitemapXml, err := createSitemapXml(group)
	if err != nil {
		log.Fatal("work failed:", err)
	}
	return
}

//Saves the sitemap from the sitemap.URLSet
func (s *SitemapGroup) Create() {
	var path string
	url_set := s.getURLSet()
	xml := s.createXML(url_set)
	sitemap_name := s.getSitemapName()
	path = s.folder + sitemap_name

	err := saveXml(xml, path)

	if err != nil {
		log.Fatal("File not saved:", err)
	}
	savedSitemaps = append(savedSitemaps, sitemap_name)
	s.group_count++

	log.Printf("Sitemap created on %s", path)

}

//Mandatory operation, handle the rest of the url that has not been added to any sitemap and add.
//Furthermore performs cleaning of variables and closes the channel group
func (s *SitemapGroup) CloseGroup() {
	s.Create()
	close(s.url_channel)
	s.Clear()
}

//Initialize channel
func (s *SitemapGroup) Initialize() {
	s.url_channel = make(chan URL)
	for entry := range s.url_channel {
		s.urls = append(s.urls, entry)
		if len(s.urls) == MAXURLSETSIZE {
			go func() {
				s.Create()
			}()
			s.Clear()
		}
	}
}

//Configure name and folder of group
func (s *SitemapGroup) Configure(name string, folder string) {
	s.name = strings.Replace(name, ".xml.gz", "", 1)
	s.group_count = 1
	_, err := ioutil.ReadDir(folder)
	if err != nil {
		log.Fatal("Dir not allowed - ", err)
	}
	s.folder = folder
}
