package sitemap

import (
	"fmt"
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

	//guards err, which is written from the Initialize goroutine and read by
	//the caller through Err()
	errMu sync.Mutex
	err   error

	//keeps a second Close from panicking on an already closed channel
	closeOnce sync.Once
}

// setErr records the first error seen, so a later success cannot mask it.
func (s *IndexGroup) setErr(err error) {
	if err == nil {
		return
	}
	s.errMu.Lock()
	defer s.errMu.Unlock()
	if s.err == nil {
		s.err = err
	}
}

// Err returns the first error encountered while writing the index files, or nil.
// Create is also called internally from the goroutine that drains Add, where
// there is no caller to return to - those errors surface only here. Check it
// after the group is closed:
//
//	<-group.Close()
//	if err := group.Err(); err != nil {
//		return err
//	}
func (s *IndexGroup) Err() error {
	s.errMu.Lock()
	defer s.errMu.Unlock()
	return s.err
}

// Add a sitemap.Sitemap to the group
func (s *IndexGroup) Add(entry Sitemap) {
	s.sitemap_channel <- entry
}

// Clean Urls not yet added to the group
func (s *IndexGroup) Clear() {
	s.sitemaps = []Sitemap{}
}

// Returns one sitemap.Index of Urls not yet added to the group
func (s *IndexGroup) getSitemapSet() Index {
	return Index{Sitemaps: s.sitemaps}
}

func (s *IndexGroup) getSitemapName() string {
	return s.name + "_" + strconv.Itoa(s.group_count) + ".xml.gz"
}

// Saves the sitemap from the sitemap.URLSet.
//
// Returns an error instead of terminating the process, as it did before
// v1.1.0. Callers that ignore the result still compile unchanged; errors
// raised on the internal path are also recorded and readable via Err.
func (s *IndexGroup) Create(index Index) error {
	var path string
	var remnant []Sitemap
	xml, err := createSitemapIndexXml(index)
	if err == ErrMaxFileSize {
		//a single entry that busts the limit cannot be split any further
		if len(index.Sitemaps) < 2 {
			return s.fail(err)
		}
		//splits the set in half recursively, preserving entry order.
		//len/2 (not MAXURLSETSIZE/2) because the file size limit can be reached
		//with far fewer than 50k entries.
		newlimit := len(index.Sitemaps) / 2
		if err := s.Create(Index{Sitemaps: index.Sitemaps[:newlimit]}); err != nil {
			return err
		}
		return s.Create(Index{Sitemaps: index.Sitemaps[newlimit:]})
	} else if err == ErrMaxUrlSetSize {
		remnant = index.Sitemaps[MAXURLSETSIZE:]
		index.Sitemaps = index.Sitemaps[:MAXURLSETSIZE]
		xml, err = createSitemapIndexXml(index)
	}

	if err != nil {
		return s.fail(err)
	}

	sitemap_name := s.getSitemapName()
	path = filepath.Join(s.folder, sitemap_name)

	if err := saveXml(xml, path); err != nil {
		return s.fail(err)
	}
	s.group_count++
	s.Clear()
	//append remnant urls if exists
	if len(remnant) > 0 {
		s.sitemaps = append(s.sitemaps, remnant...)
	}
	log.Printf("Sitemap created on %s", path)

	return nil
}

// fail records err on the group and returns it, wrapped for context.
func (s *IndexGroup) fail(err error) error {
	err = fmt.Errorf("sitemap index not saved: %w", err)
	s.setErr(err)
	return err
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

// Mandatory operation, handle the rest of the url that has not been added to any sitemap and add.
// Furthermore performs cleaning of variables and closes the channel group.
//
// Safe to call more than once - closing an already closed channel panics, and
// CloseIndexGroups makes a second call easy to arrive at by accident. Calling
// Add after Close still panics, as a send on a closed channel: finish adding
// before closing.
func (s *IndexGroup) Close() <-chan bool {
	var closeDone = make(chan bool, 1)
	s.closeOnce.Do(func() {
		close(s.sitemap_channel)
	})

	go func() {
		<-s.done
		closeDone <- true
	}()

	return closeDone
}

// Initialize channel
func (s *IndexGroup) Initialize() {
	//Configure already created these, before this goroutine was started. The
	//fallback only covers callers that invoke Initialize without Configure.
	if s.done == nil {
		s.done = make(chan bool, 1)
	}
	if s.sitemap_channel == nil {
		s.sitemap_channel = make(chan Sitemap)
	}

	//there is no caller to return to inside this goroutine, so failures are
	//both logged here and recorded on the group for Err() to report
	for entry := range s.sitemap_channel {
		s.sitemaps = append(s.sitemaps, entry)
		if len(s.sitemaps) == MAXURLSETSIZE {
			if err := s.Create(s.getSitemapSet()); err != nil {
				log.Println(err)
			}
		}
	}

	//remnant urls. Guarded: when the total is an exact multiple of
	//MAXURLSETSIZE the loop above already wrote everything, and creating again
	//would leave behind an index file with no entries in it.
	if len(s.sitemaps) > 0 {
		if err := s.Create(s.getSitemapSet()); err != nil {
			log.Println(err)
		}
	}
	s.Clear()

	//closed rather than sent to: a send only ever releases one waiter, so a
	//second Close would wait for a signal that never comes again.
	close(s.done)
}

// Configure name and folder of group
func (s *IndexGroup) Configure(name string, folder string) error {
	s.name = strings.Replace(name, ".xml.gz", "", 1)
	s.group_count = 1
	s.folder = folder

	//Created here, synchronously, rather than inside the Initialize goroutine.
	//NewIndexGroup calls Configure before starting that goroutine, so a caller
	//that reaches Add or Close first cannot race on the assignment - nor block
	//forever sending on a channel it observed as nil.
	s.done = make(chan bool, 1)
	s.sitemap_channel = make(chan Sitemap)
	//0755, not 0655: without the owner execute bit the process cannot create
	//files inside the directory it just made.
	if _, err := os.Stat(folder); err != nil {
		return os.MkdirAll(folder, 0755)
	}
	return nil
}
