package sitemap

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// collector is a writer that keeps every file it is handed.
type collector struct {
	mu    sync.Mutex
	files []File
}

func (c *collector) write(f File) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.files = append(c.files, f)
	return nil
}

func (c *collector) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.files)
}

func (c *collector) urls(t *testing.T) int {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, f := range c.files {
		n += len(locsOf(t, f.Content))
	}
	return n
}

// With a writer, files come out during Add and the group stops retaining what
// it already emitted. This is the behaviour the package had before 72b7a1a,
// where Create wrote the file and cleared the group every MAXURLSETSIZE.
func TestWriterEmitsDuringAddAndBoundsRetention(t *testing.T) {
	c := &collector{}
	g := NewSitemapGroup("bounded", false)
	g.SetWriter(c.write)

	const n = MAXURLSETSIZE * 3
	peak := 0
	for i := 0; i < n; i++ {
		g.Add(URL{Loc: fmt.Sprintf("https://ex.com/p%d", i)})

		g.mu.Lock()
		if len(g.urls) > peak {
			peak = len(g.urls)
		}
		g.mu.Unlock()
	}

	if got := c.count(); got != 3 {
		t.Errorf("got %d files emitted during Add, want 3", got)
	}
	if peak > MAXURLSETSIZE {
		t.Errorf("retained %d urls at peak, want at most %d", peak, MAXURLSETSIZE)
	}

	if err := g.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if err := g.Err(); err != nil {
		t.Fatalf("Err: %v", err)
	}
	if got := c.urls(t); got != n {
		t.Errorf("wrote %d urls, want %d", got, n)
	}
}

// Flush emits whatever did not fill a whole file.
func TestFlushEmitsRemainder(t *testing.T) {
	c := &collector{}
	g := NewSitemapGroup("remainder", false)
	g.SetWriter(c.write)

	const n = 10
	for i := 0; i < n; i++ {
		g.Add(URL{Loc: fmt.Sprintf("https://ex.com/p%d", i)})
	}
	if got := c.count(); got != 0 {
		t.Fatalf("nothing should have been emitted yet, got %d files", got)
	}

	if err := g.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if got := c.count(); got != 1 {
		t.Fatalf("got %d files after Flush, want 1", got)
	}
	if got := c.urls(t); got != n {
		t.Errorf("wrote %d urls, want %d", got, n)
	}

	// calling it again is harmless
	if err := g.Flush(); err != nil {
		t.Errorf("second Flush: %v", err)
	}
	if got := c.count(); got != 1 {
		t.Errorf("second Flush emitted another file: %d", got)
	}
}

// The reason Create must not clear the group: a concurrent Add appends while a
// batch is being written, and clearing would drop it silently.
func TestConcurrentAddDuringFlushLosesNothing(t *testing.T) {
	c := &collector{}
	g := NewSitemapGroup("concurrent", false)
	g.SetWriter(c.write)

	const n = MAXURLSETSIZE*2 + 137
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			g.Add(URL{Loc: fmt.Sprintf("https://ex.com/p%d", i)})
		}(i)
	}
	wg.Wait()

	if err := g.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if err := g.Err(); err != nil {
		t.Fatalf("Err: %v", err)
	}
	if got := c.urls(t); got != n {
		t.Errorf("wrote %d of %d urls, %d lost", got, n, n-got)
	}
}

// File numbering has to keep running across incremental flushes.
func TestWriterNamesFilesSequentially(t *testing.T) {
	c := &collector{}
	g := NewSitemapGroup("seq.xml.gz", false)
	g.SetWriter(c.write)

	for i := 0; i < MAXURLSETSIZE*2; i++ {
		g.Add(URL{Loc: fmt.Sprintf("https://ex.com/p%d", i)})
	}
	if err := g.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	var names []string
	for _, f := range c.files {
		names = append(names, f.Name)
	}
	want := []string{"seq_1.xml.gz", "seq_2.xml.gz"}
	if len(names) != len(want) {
		t.Fatalf("got %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Errorf("file %d = %q, want %q", i, names[i], want[i])
		}
	}
	// URLs() reports the same names
	if got := g.URLs(); len(got) != 2 {
		t.Errorf("URLs() = %v, want 2 names", got)
	}
}

// A writer error must not be swallowed.
func TestWriterErrorSurfacesThroughErr(t *testing.T) {
	boom := errors.New("disk full")
	g := NewSitemapGroup("failing", false)
	g.SetWriter(func(File) error { return boom })

	for i := 0; i < MAXURLSETSIZE; i++ {
		g.Add(URL{Loc: fmt.Sprintf("https://ex.com/p%d", i)})
	}

	if err := g.Err(); !errors.Is(err, boom) {
		t.Errorf("Err() = %v, want %v", err, boom)
	}
}

func TestFlushReturnsWriterError(t *testing.T) {
	boom := errors.New("nope")
	g := NewSitemapGroup("failing2", false)
	g.SetWriter(func(File) error { return boom })
	g.Add(URL{Loc: "https://ex.com/p"})

	if err := g.Flush(); !errors.Is(err, boom) {
		t.Errorf("Flush() = %v, want %v", err, boom)
	}
}

func TestFlushWithoutWriterIsAnError(t *testing.T) {
	g := NewSitemapGroup("nowriter", false)
	g.Add(URL{Loc: "https://ex.com/p"})

	err := g.Flush()
	if err == nil {
		t.Fatal("expected an error when Flush is called without SetWriter")
	}
	if !strings.Contains(err.Error(), "SetWriter") {
		t.Errorf("error should point at SetWriter, got %v", err)
	}
}

// Without a writer nothing changes: Add only accumulates, and Files stays the
// way to get the files out.
func TestWithoutWriterBehaviourIsUnchanged(t *testing.T) {
	g := NewSitemapGroup("classic", false)

	const n = MAXURLSETSIZE + 5
	for i := 0; i < n; i++ {
		g.Add(URL{Loc: fmt.Sprintf("https://ex.com/p%d", i)})
	}

	g.mu.Lock()
	retained := len(g.urls)
	g.mu.Unlock()
	if retained != n {
		t.Errorf("retained %d urls, want all %d with no writer set", retained, n)
	}

	got := 0
	files := 0
	for f := range g.Files() {
		files++
		got += len(locsOf(t, f.Content))
	}
	if files != 2 {
		t.Errorf("got %d files, want 2", files)
	}
	if got != n {
		t.Errorf("got %d urls, want %d", got, n)
	}
}

// Alternates ride along the incremental path too, and the 10mb split still
// applies inside a flush.
func TestWriterSplitsOversizedFlush(t *testing.T) {
	c := &collector{}
	g := NewSitemapGroup("i18n", false)
	g.SetWriter(c.write)

	long := strings.Repeat("a", 120)
	for i := 0; i < MAXURLSETSIZE; i++ {
		path := fmt.Sprintf("%s/%d/", long, i)
		g.Add(URL{
			Loc: "https://ex.com/" + path,
			Alternates: []Alternate{
				{HrefLang: "pt", Href: "https://ex.com/" + path},
				{HrefLang: "es", Href: "https://ex.br/" + path},
			},
		})
	}
	if err := g.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	if c.count() < 2 {
		t.Errorf("a flush over 10mb should have been split, got %d file(s)", c.count())
	}
	for _, f := range c.files {
		if len(f.Content) > MAXFILESIZE {
			t.Errorf("%s is %d bytes, over the %d limit", f.Name, len(f.Content), MAXFILESIZE)
		}
	}
	if got := c.urls(t); got != MAXURLSETSIZE {
		t.Errorf("wrote %d urls, want %d", got, MAXURLSETSIZE)
	}
}

// A blocked writer keeps a flush in flight while Adds keep arriving, which is
// the interleaving that matters: the batch has left the group, and whatever
// arrives meanwhile must not be swept away with it.
func TestAddDuringAnInFlightFlushLosesNothing(t *testing.T) {
	release := make(chan struct{})
	var (
		mu    sync.Mutex
		files []File
		once  sync.Once
	)

	g := NewSitemapGroup("inflight", false)
	g.SetWriter(func(f File) error {
		// hold only the first delivery, so the rest of the run is not serialized
		once.Do(func() { <-release })
		mu.Lock()
		files = append(files, f)
		mu.Unlock()
		return nil
	})

	const extra = 500
	const n = MAXURLSETSIZE + extra

	// Add calls the writer inline, so the goroutine that trips the flush is the
	// one that blocks. Real overlap needs other goroutines still adding, which
	// is also how a consumer with concurrent producers behaves.
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			g.Add(URL{Loc: fmt.Sprintf("https://ex.com/p%d", i)})
		}(i)
	}

	// wait until urls have piled up behind the blocked writer, then let it go
	deadline := time.Now().Add(20 * time.Second)
	for {
		g.mu.Lock()
		pending := len(g.urls)
		g.mu.Unlock()
		if pending >= extra/2 || time.Now().After(deadline) {
			break
		}
		runtime.Gosched()
	}
	close(release)
	wg.Wait()

	if err := g.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if err := g.Err(); err != nil {
		t.Fatalf("Err: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	got := 0
	for _, f := range files {
		got += len(locsOf(t, f.Content))
	}
	if got != n {
		t.Errorf("wrote %d of %d urls, %d lost while a flush was in flight", got, n, n-got)
	}
}

// A file that was built but never delivered must not be left in URLs(), or the
// index ends up naming a file nobody wrote.
func TestUndeliveredFileIsNotReportedAsWritten(t *testing.T) {
	boom := errors.New("disk full")
	g := NewSitemapGroup("phantom", false)
	g.SetWriter(func(File) error { return boom })

	for i := 0; i < MAXURLSETSIZE; i++ {
		g.Add(URL{Loc: fmt.Sprintf("https://ex.com/p%d", i)})
	}

	if err := g.Err(); !errors.Is(err, boom) {
		t.Fatalf("Err() = %v, want %v", err, boom)
	}
	if names := g.URLs(); len(names) != 0 {
		t.Errorf("URLs() = %v, want empty: none of those files were written", names)
	}
}

// Only the deliveries that failed are forgotten; earlier ones did reach disk.
func TestDeliveredFilesStayReportedWhenALaterOneFails(t *testing.T) {
	boom := errors.New("disk full")
	delivered := 0
	g := NewSitemapGroup("partial", false)
	g.SetWriter(func(File) error {
		delivered++
		if delivered > 1 {
			return boom
		}
		return nil
	})

	// long locs, so one flush splits into several files
	long := strings.Repeat("a", 260)
	for i := 0; i < MAXURLSETSIZE; i++ {
		g.Add(URL{Loc: fmt.Sprintf("https://ex.com/%s%d", long, i)})
	}

	if err := g.Err(); !errors.Is(err, boom) {
		t.Fatalf("Err() = %v, want %v", err, boom)
	}
	if names := g.URLs(); len(names) != 1 {
		t.Errorf("URLs() = %v, want just the one file that was delivered", names)
	}
}

// Files() used to swallow a build failure entirely: the channel closed with no
// file and nothing for the caller to check.
func TestFilesRecordsBuildFailures(t *testing.T) {
	g := NewSitemapGroup("toobig", false)
	g.Add(URL{Loc: "https://ex.com/" + strings.Repeat("d", MAXFILESIZE+1)})

	files := 0
	for range g.Files() {
		files++
	}
	if files != 0 {
		t.Errorf("got %d files for an unsplittable url, want 0", files)
	}
	if err := g.Err(); !errors.Is(err, ErrMaxFileSize) {
		t.Errorf("Err() = %v, want ErrMaxFileSize", err)
	}
}

// Create must return every file it made, not stash the overflow on the group:
// a caller writing what it got back would silently drop those urls.
func TestCreateReturnsACompletePartition(t *testing.T) {
	g := NewSitemapGroup("partition", false)

	urls := make([]URL, MAXURLSETSIZE+137)
	for i := range urls {
		urls[i] = URL{Loc: fmt.Sprintf("https://ex.com/p%d", i)}
	}

	files, err := g.Create(URLSet{URLs: urls})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("got %d files, want 2", len(files))
	}

	total := 0
	for _, f := range files {
		total += len(locsOf(t, f.Content))
	}
	if total != len(urls) {
		t.Errorf("returned files hold %d of %d urls", total, len(urls))
	}

	g.mu.Lock()
	left := len(g.urls)
	g.mu.Unlock()
	if left != 0 {
		t.Errorf("Create left %d urls on the group instead of returning them", left)
	}
}
