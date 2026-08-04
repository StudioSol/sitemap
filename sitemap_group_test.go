package sitemap

import (
	"encoding/xml"
	"fmt"
	"strings"
	"sync"
	"testing"
)

// locsOf parses a generated sitemap and returns its <loc> values in order.
func locsOf(t *testing.T, content []byte) []string {
	t.Helper()
	var set struct {
		URLs []struct {
			Loc string `xml:"loc"`
		} `xml:"url"`
	}
	if err := xml.Unmarshal(content, &set); err != nil {
		t.Fatalf("unmarshal generated sitemap: %v", err)
	}
	locs := make([]string, 0, len(set.URLs))
	for _, u := range set.URLs {
		locs = append(locs, u.Loc)
	}
	return locs
}

// A url set can exceed the 10mb file limit with far fewer than 50k urls -
// alternates make that the common case. Splitting on MAXURLSETSIZE/2 used to
// panic with "slice bounds out of range".
func TestFilesSplitsOversizedSetWithFewUrls(t *testing.T) {
	g := NewSitemapGroup("big", false)

	const n = 2000
	long := strings.Repeat("a", 27000)
	for i := 0; i < n; i++ {
		g.Add(URL{Loc: fmt.Sprintf("https://ex.com/%s%d", long, i)})
	}

	var files []File
	for f := range g.Files() {
		files = append(files, f)
	}

	if len(files) < 2 {
		t.Fatalf("expected the oversized set to be split, got %d file(s)", len(files))
	}

	total := 0
	for _, f := range files {
		if len(f.Content) > MAXFILESIZE {
			t.Errorf("%s is %d bytes, over the %d limit", f.Name, len(f.Content), MAXFILESIZE)
		}
		total += len(locsOf(t, f.Content))
	}
	if total != n {
		t.Errorf("got %d urls across all files, want %d", total, n)
	}
}

// The same oversize path, driven by alternates rather than by long locs.
func TestFilesSplitsOversizedSetCausedByAlternates(t *testing.T) {
	g := NewSitemapGroup("i18n", false)

	const n = 3000
	for i := 0; i < n; i++ {
		alts := make([]Alternate, 0, 20)
		for j := 0; j < 20; j++ {
			alts = append(alts, Alternate{
				HrefLang: fmt.Sprintf("l%d", j),
				Href:     fmt.Sprintf("https://ex.com/%s/l%d/p%d", strings.Repeat("b", 800), j, i),
			})
		}
		g.Add(URL{Loc: fmt.Sprintf("https://ex.com/p%d", i), Alternates: alts})
	}

	var files []File
	for f := range g.Files() {
		files = append(files, f)
	}

	if len(files) < 2 {
		t.Fatalf("expected split, got %d file(s)", len(files))
	}
	total := 0
	for _, f := range files {
		if len(f.Content) > MAXFILESIZE {
			t.Errorf("%s is %d bytes, over the %d limit", f.Name, len(f.Content), MAXFILESIZE)
		}
		if !strings.Contains(string(f.Content), "xmlns:xhtml") {
			t.Errorf("%s lost the xhtml namespace after the split", f.Name)
		}
		total += len(locsOf(t, f.Content))
	}
	if total != n {
		t.Errorf("got %d urls across all files, want %d", total, n)
	}
}

// Splitting must not shuffle urls: the concatenation of all files has to match
// the order they were added in.
func TestSplitPreservesUrlOrder(t *testing.T) {
	g := NewSitemapGroup("ordered", false)

	const n = 4000
	long := strings.Repeat("c", 14000)
	want := make([]string, 0, n)
	for i := 0; i < n; i++ {
		loc := fmt.Sprintf("https://ex.com/%s%05d", long, i)
		want = append(want, loc)
		g.Add(URL{Loc: loc})
	}

	var got []string
	files := 0
	for f := range g.Files() {
		files++
		got = append(got, locsOf(t, f.Content)...)
	}
	if files < 2 {
		t.Fatalf("expected split, got %d file(s)", files)
	}
	if len(got) != len(want) {
		t.Fatalf("got %d urls, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("url order broken at index %d:\ngot  ...%s\nwant ...%s",
				i, got[i][len(got[i])-8:], want[i][len(want[i])-8:])
		}
	}
}

// A single url too big to fit must surface an error rather than recurse forever.
func TestCreateSingleOversizedUrlReturnsError(t *testing.T) {
	g := NewSitemapGroup("huge", false)
	_, err := g.Create(URLSet{URLs: []URL{{Loc: "https://ex.com/" + strings.Repeat("d", MAXFILESIZE+1)}}})
	if err != ErrMaxFileSize {
		t.Fatalf("got err %v, want ErrMaxFileSize", err)
	}
}

func TestFilesSplitsOnUrlCount(t *testing.T) {
	g := NewSitemapGroup("count", false)

	n := MAXURLSETSIZE + 10
	for i := 0; i < n; i++ {
		g.Add(URL{Loc: fmt.Sprintf("https://ex.com/p%d", i)})
	}

	total := 0
	files := 0
	for f := range g.Files() {
		files++
		locs := locsOf(t, f.Content)
		if len(locs) > MAXURLSETSIZE {
			t.Errorf("%s holds %d urls, over the %d limit", f.Name, len(locs), MAXURLSETSIZE)
		}
		total += len(locs)
	}
	if files != 2 {
		t.Errorf("got %d files, want 2", files)
	}
	if total != n {
		t.Errorf("got %d urls total, want %d", total, n)
	}
}

// Add is called from one goroutine per url by real consumers. An
// unsynchronized append does not merely race - it silently drops entries, and
// a sitemap that is quietly incomplete is worse than one that fails loudly.
func TestConcurrentAddKeepsEveryUrl(t *testing.T) {
	g := NewSitemapGroup("concurrent", false)

	const n = 5000
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			g.Add(URL{Loc: fmt.Sprintf("https://ex.com/p%d", i)})
		}(i)
	}
	wg.Wait()

	got := 0
	for f := range g.Files() {
		got += len(locsOf(t, f.Content))
	}
	if got != n {
		t.Fatalf("wrote %d of %d urls, %d silently dropped", got, n, n-got)
	}
}

// Clear must not race with a concurrent Add either.
func TestConcurrentAddAndClear(t *testing.T) {
	g := NewSitemapGroup("churn", false)

	var wg sync.WaitGroup
	for i := 0; i < 500; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			g.Add(URL{Loc: fmt.Sprintf("https://ex.com/p%d", i)})
		}(i)
		go func() {
			defer wg.Done()
			g.Clear()
		}()
	}
	wg.Wait()
}

func TestGroupNamesAreSequential(t *testing.T) {
	g := NewSitemapGroup("blog.xml.gz", false)
	for i := 0; i < MAXURLSETSIZE+1; i++ {
		g.Add(URL{Loc: fmt.Sprintf("https://ex.com/p%d", i)})
	}
	var names []string
	for f := range g.Files() {
		names = append(names, f.Name)
	}
	if len(names) != 2 || names[0] != "blog_1.xml.gz" || names[1] != "blog_2.xml.gz" {
		t.Errorf("got names %v, want [blog_1.xml.gz blog_2.xml.gz]", names)
	}
	if urls := g.URLs(); len(urls) != 2 {
		t.Errorf("URLs() returned %v, want 2 entries", urls)
	}
}
