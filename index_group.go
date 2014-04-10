package sitemap

import (
	"io/ioutil"
	"log"
	"strconv"
	"strings"
)

type IndexGroup struct {
	name            string
	folder          string
	group_count     int
	sitemaps        []Sitemap
	sitemap_channel chan Sitemap
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

//Create index sitemap XML from a Index
func (s *IndexGroup) createXML(group Index) (indexXml []byte) {
	indexXml, err := createSitemapIndexXml(group)
	if err != nil {
		log.Fatal("work failed:", err)
	}
	return
}

//Saves the sitemap from the sitemap.URLSet
func (s *IndexGroup) Create(index Index) {
	var path string
	xml := s.createXML(index)
	sitemap_name := s.getSitemapName()
	path = s.folder + sitemap_name

	err := saveXml(xml, path)

	if err != nil {
		log.Fatal("File not saved:", err)
	}

	s.group_count++

	log.Printf("Sitemap created on %s", path)

}

//Mandatory operation, handle the rest of the url that has not been added to any sitemap and add.
//Furthermore performs cleaning of variables and closes the channel group
func (s *IndexGroup) CloseGroup() {
	s.Create(s.getSitemapSet())
	close(s.sitemap_channel)
	s.Clear()
}

//Initialize channel
func (s *IndexGroup) Initialize() {
	s.sitemap_channel = make(chan Sitemap)
	for entry := range s.sitemap_channel {
		s.sitemaps = append(s.sitemaps, entry)
		if len(s.sitemaps) == MAXURLSETSIZE {
			go func(index Index) {
				s.Create(index)
			}(s.getSitemapSet())
			s.Clear()
		}
	}
}

//Configure name and folder of group
func (s *IndexGroup) Configure(name string, folder string) {
	s.name = strings.Replace(name, ".xml.gz", "", 1)
	s.group_count = 1
	_, err := ioutil.ReadDir(folder)
	if err != nil {
		log.Fatal("Dir not allowed - ", err)
	}
	s.folder = folder
}
