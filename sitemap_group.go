package sitemap

import (
	"compress/gzip"
	"io"
	"log"
	"strconv"
	"strings"
	"sync"
)

type File struct {
	Name    string
	Content []byte
}

func (f *File) Write(w io.Writer) error {
	zip, err := gzip.NewWriterLevel(w, gzip.BestCompression)
	if err != nil {
		return err
	}
	if _, err := zip.Write(f.Content); err != nil {
		zip.Close()
		return err
	}
	//Close writes the gzip footer, so its error is the difference between a
	//complete file and a truncated one
	return zip.Close()
}

type SitemapGroup struct {
	name          string
	folder        string
	group_count   int
	urls          []URL
	isMobile      bool
	savedSitemaps []string

	//guards urls. Add is routinely called from many goroutines at once, and an
	//unsynchronized append silently drops entries.
	mu sync.Mutex
}

// Add a sitemap.URL to the group. Safe to call concurrently.
func (s *SitemapGroup) Add(url URL) {
	s.mu.Lock()
	s.urls = append(s.urls, url)
	s.mu.Unlock()
}

// Clean Urls not yet added to the group
func (s *SitemapGroup) Clear() {
	s.mu.Lock()
	s.urls = nil
	s.mu.Unlock()
}

// takeUrls removes and returns every url pending in the group.
func (s *SitemapGroup) takeUrls() []URL {
	s.mu.Lock()
	defer s.mu.Unlock()
	urls := s.urls
	s.urls = nil
	return urls
}

// addAll appends urls back onto the group, for remnants left by a split.
func (s *SitemapGroup) addAll(urls []URL) {
	s.mu.Lock()
	s.urls = append(s.urls, urls...)
	s.mu.Unlock()
}

func (s *SitemapGroup) getSitemapName() string {
	return s.name + "_" + strconv.Itoa(s.group_count) + ".xml.gz"
}

// Saves the sitemap from the sitemap.URLSet
func (s *SitemapGroup) Create(url_set URLSet) ([]File, error) {
	var remnant []URL
	xml, err := createSitemapXml(url_set, s.isMobile)

	if err == ErrMaxFileSize {
		//a single url that busts the limit cannot be split any further
		if len(url_set.URLs) < 2 {
			return nil, err
		}
		//splits the set in half recursively, preserving url order.
		//splitting on len/2 (not MAXURLSETSIZE/2) matters because a set can
		//exceed 10mb with far fewer than 50k urls - alternates make that common.
		newlimit := len(url_set.URLs) / 2
		firstSplit, err := s.Create(URLSet{URLs: url_set.URLs[:newlimit]})
		if err != nil {
			return nil, err
		}
		secondSplit, err := s.Create(URLSet{URLs: url_set.URLs[newlimit:]})
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
		s.addAll(remnant)
	}

	return files, nil
}

// clean array of already generated sitemaps (not delete files)
func (s *SitemapGroup) ClearSavedSitemaps() {
	s.savedSitemaps = []string{}
}

// returns the url of already generated sitemaps
func (s *SitemapGroup) URLs() []string {
	return s.savedSitemaps
}

func (s *SitemapGroup) Files() chan File {
	filesChannel := make(chan File)
	go func() {
		var partialGroup []URL
		defer close(filesChannel)
		//taken under the lock, so a concurrent Add cannot race with this range
		for _, entry := range s.takeUrls() {
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

// Configure name and folder of group
func (s *SitemapGroup) Configure(name string, isMobile bool) {
	s.name = strings.Replace(name, ".xml.gz", "", 1)
	s.group_count = 1
	s.isMobile = isMobile
}
