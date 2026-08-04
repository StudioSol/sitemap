package sitemap

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Create used to call log.Fatal here, taking the host process down with it.
// It must report the failure instead.
func TestCreateReturnsErrorOnUnwritablePath(t *testing.T) {
	g := new(IndexGroup)
	if err := g.Configure("idx", filepath.Join(t.TempDir(), "does", "not", "exist")); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	// Configure creates the folder, so remove it to force saveXml to fail
	if err := os.RemoveAll(g.folder); err != nil {
		t.Fatalf("RemoveAll: %v", err)
	}

	err := g.Create(Index{Sitemaps: []Sitemap{{Loc: "https://ex.com/s_1.xml.gz"}}})
	if err == nil {
		t.Fatal("expected an error for an unwritable folder, got nil")
	}
	if !strings.Contains(err.Error(), "sitemap index not saved") {
		t.Errorf("error lost its context: %v", err)
	}
	// the underlying cause must stay reachable through errors.Is/As
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected the wrapped cause to be os.ErrNotExist, got %v", err)
	}
	if g.Err() == nil {
		t.Error("Err() should expose the failure recorded by Create")
	}
}

// A single index entry too big to fit cannot be split, so it must surface
// ErrMaxFileSize rather than recurse forever or abort the process.
func TestCreateSingleOversizedEntryReturnsError(t *testing.T) {
	g := new(IndexGroup)
	if err := g.Configure("idx", t.TempDir()); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	err := g.Create(Index{Sitemaps: []Sitemap{
		{Loc: "https://ex.com/" + strings.Repeat("a", MAXFILESIZE+1)},
	}})
	if !errors.Is(err, ErrMaxFileSize) {
		t.Fatalf("got %v, want ErrMaxFileSize", err)
	}
}

func TestErrIsNilOnSuccess(t *testing.T) {
	dir := t.TempDir()
	g := new(IndexGroup)
	if err := g.Configure("idx", dir); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	if err := g.Create(Index{Sitemaps: []Sitemap{{Loc: "https://ex.com/s_1.xml.gz"}}}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := g.Err(); err != nil {
		t.Errorf("Err() = %v, want nil", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "idx_1.xml.gz")); err != nil {
		t.Errorf("expected index file to exist: %v", err)
	}
}

// Err records the first failure and does not let a later success clear it.
func TestErrKeepsFirstFailure(t *testing.T) {
	g := new(IndexGroup)
	first := errors.New("first")
	g.setErr(first)
	g.setErr(errors.New("second"))
	g.setErr(nil)

	if g.Err() != first {
		t.Errorf("Err() = %v, want the first error", g.Err())
	}
}

// Errors raised inside the Add/Initialize goroutine have no caller to return
// to; they must still be readable after the group is closed.
func TestErrSurfacesFailureFromInitializeGoroutine(t *testing.T) {
	dir := t.TempDir()
	g, err := NewIndexGroup(dir, "idx")
	if err != nil {
		t.Fatalf("NewIndexGroup: %v", err)
	}

	// make the output folder unwritable so the internal Create fails
	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("RemoveAll: %v", err)
	}

	g.Add(Sitemap{Loc: "https://ex.com/s_1.xml.gz"})
	<-g.Close()

	if g.Err() == nil {
		t.Fatal("Err() should expose the failure from the Initialize goroutine")
	}
}

// NewIndexGroup starts Initialize in a goroutine, but the channels it uses are
// created by Configure beforehand. If they were created inside that goroutine,
// Add would race on the assignment - and deadlock outright whenever it observed
// the channel as nil, because a send on a nil channel blocks forever.
func TestChannelsReadyBeforeInitializeGoroutineRuns(t *testing.T) {
	dir := t.TempDir()
	g, err := NewIndexGroup(dir, "idx")
	if err != nil {
		t.Fatalf("NewIndexGroup: %v", err)
	}

	if g.sitemap_channel == nil {
		t.Fatal("sitemap_channel is nil right after NewIndexGroup; Add would block forever")
	}
	if g.done == nil {
		t.Fatal("done is nil right after NewIndexGroup; Close would block forever")
	}

	// exercise the full path with no scheduling grace for the goroutine
	g.Add(Sitemap{Loc: "https://ex.com/s_1.xml.gz"})
	<-g.Close()

	if err := g.Err(); err != nil {
		t.Fatalf("Err() = %v, want nil", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "idx_1.xml.gz")); err != nil {
		t.Errorf("expected index file to exist: %v", err)
	}
}

// Configure must produce a directory the process can actually write into;
// it used to be created as 0655, which lacks the owner execute bit.
func TestConfigureCreatesWritableFolder(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sitemaps")
	g := new(IndexGroup)
	if err := g.Configure("idx", dir); err != nil {
		t.Fatalf("Configure: %v", err)
	}

	f, err := os.Create(filepath.Join(dir, "probe"))
	if err != nil {
		t.Fatalf("cannot write into the folder Configure created: %v", err)
	}
	f.Close()
}

// A total that is an exact multiple of MAXURLSETSIZE used to leave an extra
// index file behind with no entries in it.
func TestNoEmptyIndexAtExactBoundary(t *testing.T) {
	dir := t.TempDir()
	g, err := NewIndexGroup(dir, "idx")
	if err != nil {
		t.Fatalf("NewIndexGroup: %v", err)
	}

	for i := 0; i < MAXURLSETSIZE; i++ {
		g.Add(Sitemap{Loc: fmt.Sprintf("https://ex.com/s_%d.xml.gz", i)})
	}
	<-g.Close()

	if err := g.Err(); err != nil {
		t.Fatalf("Err: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("got %d index files (%v), want 1 - the second would be empty", len(entries), names)
	}
}

// CloseIndexGroups makes a second Close easy to reach by accident, and closing
// an already closed channel panics.
func TestCloseTwiceDoesNotPanic(t *testing.T) {
	g, err := NewIndexGroup(t.TempDir(), "idx")
	if err != nil {
		t.Fatalf("NewIndexGroup: %v", err)
	}
	g.Add(Sitemap{Loc: "https://ex.com/s_1.xml.gz"})

	<-g.Close()
	<-g.Close() // would panic on send/close of a closed channel
}
