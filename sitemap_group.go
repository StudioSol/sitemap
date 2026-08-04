package sitemap

import (
	"compress/gzip"
	"errors"
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
	name             string
	folder           string
	group_count      int
	urls             []URL
	isMobile         bool
	schemaValidation bool
	savedSitemaps    []string

	//guards urls. Add is routinely called from many goroutines at once, and an
	//unsynchronized append silently drops entries.
	mu sync.Mutex

	//writer, when set, turns Add into an incremental flush: see SetWriter.
	writer func(File) error
	//serializes emit, so file numbering stays deterministic and the writer is
	//never called from two goroutines at once. Held separately from mu so that
	//Add does not block on a slow writer.
	emitMu sync.Mutex

	//guards group_count and savedSitemaps, so a name is never handed out twice
	//even when Create runs from more than one goroutine.
	metaMu sync.Mutex

	errMu sync.Mutex
	err   error
}

// nextName reserves the next file name and records it, atomically.
func (s *SitemapGroup) nextName() string {
	s.metaMu.Lock()
	defer s.metaMu.Unlock()
	name := s.name + "_" + strconv.Itoa(s.group_count) + ".xml.gz"
	s.savedSitemaps = append(s.savedSitemaps, name)
	s.group_count++
	return name
}

// forgetNames drops names from savedSitemaps, for files that were built but
// never delivered. The counter is left alone: reusing a number could collide
// with a file that did make it out.
func (s *SitemapGroup) forgetNames(names []string) {
	if len(names) == 0 {
		return
	}
	drop := make(map[string]bool, len(names))
	for _, n := range names {
		drop[n] = true
	}

	s.metaMu.Lock()
	defer s.metaMu.Unlock()
	kept := s.savedSitemaps[:0]
	for _, n := range s.savedSitemaps {
		if !drop[n] {
			kept = append(kept, n)
		}
	}
	s.savedSitemaps = kept
}

// SetWriter makes the group hand each sitemap file to fn as soon as it fills,
// instead of retaining every url until Files is called. Peak memory then stays
// around MAXURLSETSIZE urls rather than growing with the whole set - which for
// a few million urls is the difference between hundreds of megabytes and
// several gigabytes.
//
// Call Flush when there are no more urls, to emit the remainder, and Err to
// find out whether any write failed:
//
//	group.SetWriter(func(f sitemap.File) error {
//		return writeToDisk(f)
//	})
//	for _, u := range urls {
//		group.Add(u)
//	}
//	if err := group.Flush(); err != nil {
//		return err
//	}
//
// SetWriter and Files are alternative paths and should not be mixed on one
// group: with a writer set, the files are emitted through fn, so Files would
// only ever see whatever has not been flushed yet.
//
// fn is never called from two goroutines at once. Under concurrent Add the file
// numbers do not track the order urls arrived in, though - two batches can
// reach the writer in either order. Which file a url lands in carries no
// meaning for the protocol, so this only affects the names.
func (s *SitemapGroup) SetWriter(fn func(File) error) {
	s.writer = fn
}

// Flush emits the urls still pending as one or more files, through the writer
// set by SetWriter. Required once at the end; harmless to call again.
func (s *SitemapGroup) Flush() error {
	if s.writer == nil {
		return errors.New("sitemap: Flush requires SetWriter")
	}
	pending := s.takeUrls()
	if len(pending) == 0 {
		return nil
	}
	return s.emit(pending)
}

// emit builds the files for urls and hands them to the writer, one flush at a
// time.
//
// A name is only allowed to stay in URLs() once its file has been delivered:
// otherwise a failed write would leave the index pointing at a file nobody
// wrote.
func (s *SitemapGroup) emit(urls []URL) error {
	s.emitMu.Lock()
	defer s.emitMu.Unlock()

	files, err := s.Create(URLSet{URLs: urls})
	if err != nil {
		return err
	}
	for i, f := range files {
		if err := s.writer(f); err != nil {
			undelivered := make([]string, 0, len(files)-i)
			for _, rest := range files[i:] {
				undelivered = append(undelivered, rest.Name)
			}
			s.forgetNames(undelivered)
			return err
		}
	}
	return nil
}

func (s *SitemapGroup) setErr(err error) {
	if err == nil {
		return
	}
	s.errMu.Lock()
	defer s.errMu.Unlock()
	if s.err == nil {
		s.err = err
	}
}

// Err returns the first error a writer returned, or nil. Add cannot report one
// itself, so this is where failures from the incremental path surface.
func (s *SitemapGroup) Err() error {
	s.errMu.Lock()
	defer s.errMu.Unlock()
	return s.err
}

// SetSchemaValidation makes the generated sitemaps carry xsi:schemaLocation,
// pairing the sitemaps.org and xhtml namespaces with the schemas that describe
// them, so the output can be checked against those schemas.
//
// The namespace declarations themselves are unchanged, so validation mode is
// safe to inspect: it adds attributes rather than moving elements to a
// different namespace.
func (s *SitemapGroup) SetSchemaValidation(on bool) {
	s.schemaValidation = on
}

// Add a sitemap.URL to the group. Safe to call concurrently.
//
// With a writer set by SetWriter, this also emits a file every time the group
// fills, so the urls added so far can be released. Any error the writer returns
// is recorded and readable through Err.
func (s *SitemapGroup) Add(url URL) {
	s.mu.Lock()
	s.urls = append(s.urls, url)
	var batch []URL
	if s.writer != nil && len(s.urls) >= MAXURLSETSIZE {
		//taken here, under the lock, so concurrent Adds keep filling a fresh
		//slice while this batch is written
		batch, s.urls = s.urls, nil
	}
	s.mu.Unlock()

	if batch != nil {
		s.setErr(s.emit(batch))
	}
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

// Saves the sitemap from the sitemap.URLSet
func (s *SitemapGroup) Create(url_set URLSet) ([]File, error) {
	var remnant []URL
	xml, err := createSitemapXml(url_set, s.isMobile, s.schemaValidation)

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
		xml, err = createSitemapXml(url_set, s.isMobile, s.schemaValidation)
	}
	if err != nil {
		return nil, err
	}

	files := []File{
		{Name: s.nextName(), Content: xml},
	}

	//Deliberately does not clear s.urls. Create is given the set to write, and
	//callers hand it a set they already took out of the group. Clearing here
	//discarded whatever a concurrent Add had appended in the meantime, which is
	//silent url loss on the incremental path.

	//Anything past the 50k count limit becomes further files, returned along
	//with the first. Stashing the remnant back on the group instead would make
	//Create return an incomplete partition, so a caller writing what it got
	//back would quietly drop those urls.
	if len(remnant) > 0 {
		rest, err := s.Create(URLSet{URLs: remnant})
		if err != nil {
			return nil, err
		}
		files = append(files, rest...)
	}

	return files, nil
}

// clean array of already generated sitemaps (not delete files)
func (s *SitemapGroup) ClearSavedSitemaps() {
	s.metaMu.Lock()
	s.savedSitemaps = []string{}
	s.metaMu.Unlock()
}

// returns the url of already generated sitemaps
func (s *SitemapGroup) URLs() []string {
	s.metaMu.Lock()
	defer s.metaMu.Unlock()
	return append([]string(nil), s.savedSitemaps...)
}

// Files builds the pending urls into files and streams them.
//
// The channel closing does not mean everything succeeded: a batch that fails to
// build is logged and recorded, so check Err after draining. Alternative to
// SetWriter - use one or the other on a group, not both.
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
					//dropping these urls is bad enough without also hiding it,
					//and partialGroup has to reset or it never fills again
					log.Println(err)
					s.setErr(err)
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
				s.setErr(err)
			}
			for _, file := range groupFiles {
				filesChannel <- file
			}
			//no Clear here: takeUrls already emptied the group, so this would
			//only ever discard what a concurrent Add appended since
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
