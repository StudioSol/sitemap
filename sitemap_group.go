package sitemap

import (
	"compress/gzip"
	"io"
	"log"
	"strconv"
	"strings"
)

type File struct {
	Name    string
	Content []byte
}

func (f *File) Write(w io.Writer) error {
	zip, _ := gzip.NewWriterLevel(w, gzip.BestCompression)
	defer zip.Close()
	_, err := zip.Write(f.Content)
	return err
}

type SitemapGroup struct {
	name          string
	folder        string
	group_count   int
	urls          []URL
	isMobile      bool
	savedSitemaps []string
}

//Add a sitemap.URL to the group
func (s *SitemapGroup) Add(url URL) {
	s.urls = append(s.urls, url)
}

//Clean Urls not yet added to the group
func (s *SitemapGroup) Clear() {
	s.urls = nil
}

func (s *SitemapGroup) getSitemapName() string {
	return s.name + "_" + strconv.Itoa(s.group_count) + ".xml.gz"
}

//Saves the sitemap from the sitemap.URLSet
func (s *SitemapGroup) Create(url_set URLSet) ([]File, error) {
	var remnant []URL
	xml, err := createSitemapXml(url_set, s.isMobile)

	if err == ErrMaxFileSize {
		//splits into two sitemaps recursively
		newlimit := MAXURLSETSIZE / 2
		firstSplit, err := s.Create(URLSet{URLs: url_set.URLs[newlimit:]})
		if err != nil {
			return nil, err
		}
		secondSplit, err := s.Create(URLSet{URLs: url_set.URLs[:newlimit]})
		if err != nil {
			return nil, err
		}
		return append(firstSplit, secondSplit...), nil
	} else if err == ErrMaxUrlSetSize {
		remnant = url_set.URLs[MAXURLSETSIZE:]
		url_set.URLs = url_set.URLs[:MAXURLSETSIZE]
		xml, err = createSitemapXml(url_set, s.isMobile)
	}
	if err != nil {
		return nil, err
	}

	sitemap_name := s.getSitemapName()
	files := []File{
		{Name: sitemap_name, Content: xml},
	}

	s.savedSitemaps = append(s.savedSitemaps, sitemap_name)
	s.group_count++
	s.Clear()

	// append remnant urls if exists
	if len(remnant) > 0 {
		s.urls = append(s.urls, remnant...)
	}

	return files, nil
}

//clean array of already generated sitemaps (not delete files)
func (s *SitemapGroup) ClearSavedSitemaps() {
	s.savedSitemaps = []string{}
}

//returns the url of already generated sitemaps
func (s *SitemapGroup) URLs() []string {
	return s.savedSitemaps
}

func (s *SitemapGroup) Files() chan File {
	filesChannel := make(chan File)
	go func() {
		var partialGroup []URL
		defer close(filesChannel)
		for _, entry := range s.urls {
			partialGroup = append(partialGroup, entry)
			if len(partialGroup) == MAXURLSETSIZE {
				groupFiles, err := s.Create(URLSet{URLs: partialGroup})
				if err != nil {
					continue
				}
				for _, file := range groupFiles {
					filesChannel <- file
				}
				partialGroup = nil
			}
		}
		// remaining files
		if len(partialGroup) > 0 {
			groupFiles, err := s.Create(URLSet{URLs: partialGroup})
			if err != nil {
				log.Println(err)
			}
			for _, file := range groupFiles {
				filesChannel <- file
			}
			s.Clear()
		}
	}()
	return filesChannel
}

//Configure name and folder of group
func (s *SitemapGroup) Configure(name string, isMobile bool) {
	s.name = strings.Replace(name, ".xml.gz", "", 1)
	s.group_count = 1
	s.isMobile = isMobile
}
