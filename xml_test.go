package sitemap

import (
	"encoding/xml"
	"strings"
	"testing"
	"time"
)

func TestAlternatesRenderXhtmlLinks(t *testing.T) {
	got, err := createSitemapXml(URLSet{URLs: []URL{
		{
			Loc: "https://www.example.com/english/page.html",
			Alternates: []Alternate{
				{HrefLang: "de", Href: "https://www.example.com/deutsch/page.html"},
				{HrefLang: "de-ch", Href: "https://www.example.com/schweiz-deutsch/page.html"},
				{HrefLang: "en", Href: "https://www.example.com/english/page.html"},
			},
		},
	}}, false, false)
	if err != nil {
		t.Fatalf("createSitemapXml: %v", err)
	}

	s := string(got)
	if !strings.Contains(s, `xmlns:xhtml="`+XMLNSXHTML+`"`) {
		t.Errorf("missing xhtml namespace declaration in:\n%s", s)
	}
	for _, want := range []string{
		`<xhtml:link rel="alternate" hreflang="de" href="https://www.example.com/deutsch/page.html">`,
		`<xhtml:link rel="alternate" hreflang="de-ch" href="https://www.example.com/schweiz-deutsch/page.html">`,
		`<xhtml:link rel="alternate" hreflang="en" href="https://www.example.com/english/page.html">`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing alternate link %q in:\n%s", want, s)
		}
	}

	// rel is fixed by the spec and must never be emitted empty
	if strings.Contains(s, `rel=""`) {
		t.Errorf("emitted empty rel attribute in:\n%s", s)
	}
}

func TestAlternatesComeRightAfterLoc(t *testing.T) {
	now := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
	got, err := createSitemapXml(URLSet{URLs: []URL{
		{
			Loc:        "https://ex.com/en/p",
			Alternates: []Alternate{{HrefLang: "pt-br", Href: "https://ex.com/pt/p"}},
			LastMod:    &now,
			ChangeFreq: Daily,
			Priority:   0.8,
		},
	}}, false, false)
	if err != nil {
		t.Fatalf("createSitemapXml: %v", err)
	}

	s := string(got)
	loc := strings.Index(s, "<loc>")
	link := strings.Index(s, "<xhtml:link")
	lastmod := strings.Index(s, "<lastmod>")
	if !(loc < link && link < lastmod) {
		t.Errorf("expected loc < xhtml:link < lastmod, got %d/%d/%d in:\n%s", loc, link, lastmod, s)
	}
}

func TestXDefaultAlternate(t *testing.T) {
	got, err := createSitemapXml(URLSet{URLs: []URL{
		{Loc: "https://ex.com/p", Alternates: []Alternate{{HrefLang: XDefault, Href: "https://ex.com/p"}}},
	}}, false, false)
	if err != nil {
		t.Fatalf("createSitemapXml: %v", err)
	}
	if !strings.Contains(string(got), `hreflang="x-default"`) {
		t.Errorf("missing x-default hreflang in:\n%s", got)
	}
}

// A sitemap without alternates must not declare the xhtml namespace, so output
// for existing callers stays byte-identical.
func TestNoAlternatesNoXhtmlNamespace(t *testing.T) {
	got, err := createSitemapXml(URLSet{URLs: []URL{{Loc: "https://ex.com/p"}}}, false, false)
	if err != nil {
		t.Fatalf("createSitemapXml: %v", err)
	}
	s := string(got)
	if strings.Contains(s, "xhtml") {
		t.Errorf("unexpected xhtml namespace in:\n%s", s)
	}
	want := PREAMBLE + `<urlset xmlns="` + XMLNS + `"><url><loc>https://ex.com/p</loc></url></urlset>`
	if s != want {
		t.Errorf("output changed for alternate-free sitemap:\ngot  %s\nwant %s", s, want)
	}
}

func TestMobileStillRenders(t *testing.T) {
	got, err := createSitemapXml(URLSet{URLs: []URL{{Loc: "https://ex.com/p", Mobile: ISMOBILE}}}, true, false)
	if err != nil {
		t.Fatalf("createSitemapXml: %v", err)
	}
	s := string(got)
	if !strings.Contains(s, `xmlns:mobile="`+XMLNSMOBILE+`"`) || !strings.Contains(s, "<mobile:mobile>") {
		t.Errorf("mobile markup missing in:\n%s", s)
	}
}

func TestMobileAndAlternatesTogether(t *testing.T) {
	got, err := createSitemapXml(URLSet{URLs: []URL{
		{
			Loc:        "https://ex.com/p",
			Alternates: []Alternate{{HrefLang: "pt-br", Href: "https://ex.com/pt/p"}},
			Mobile:     ISMOBILE,
		},
	}}, true, false)
	if err != nil {
		t.Fatalf("createSitemapXml: %v", err)
	}
	s := string(got)
	if !strings.Contains(s, `xmlns:mobile=`) || !strings.Contains(s, `xmlns:xhtml=`) {
		t.Errorf("expected both namespaces in:\n%s", s)
	}
}

func TestSitemapXmlIsWellFormed(t *testing.T) {
	got, err := createSitemapXml(URLSet{URLs: []URL{
		{Loc: "https://ex.com/en/p", Alternates: []Alternate{
			{HrefLang: "pt-br", Href: "https://ex.com/pt/p"},
			{HrefLang: XDefault, Href: "https://ex.com/p"},
		}},
	}}, false, false)
	if err != nil {
		t.Fatalf("createSitemapXml: %v", err)
	}

	// a real parser must accept it, prefixes included
	dec := xml.NewDecoder(strings.NewReader(string(got)))
	links := 0
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		if se, ok := tok.(xml.StartElement); ok && se.Name.Local == "link" {
			links++
			if se.Name.Space != XMLNSXHTML {
				t.Errorf("link resolved to namespace %q, want %q", se.Name.Space, XMLNSXHTML)
			}
		}
	}
	if links != 2 {
		t.Errorf("parsed %d xhtml:link elements, want 2", links)
	}
}

// A caller pinned to an older index schema must keep it. Some consumers still
// publish sitemap/0.84, and forcing 0.9 would rewrite it silently.
func TestIndexKeepsCallerXMLNS(t *testing.T) {
	const legacy = "http://www.google.com/schemas/sitemap/0.84"
	got, err := createSitemapIndexXml(Index{
		XMLNS:    legacy,
		Sitemaps: []Sitemap{{Loc: "https://ex.com/s_1.xml.gz"}},
	})
	if err != nil {
		t.Fatalf("createSitemapIndexXml: %v", err)
	}
	if !strings.Contains(string(got), `xmlns="`+legacy+`"`) {
		t.Errorf("caller XMLNS was overwritten:\n%s", got)
	}
}

func TestIndexDefaultsXMLNSWhenEmpty(t *testing.T) {
	got, err := createSitemapIndexXml(Index{Sitemaps: []Sitemap{{Loc: "https://ex.com/s_1.xml.gz"}}})
	if err != nil {
		t.Fatalf("createSitemapIndexXml: %v", err)
	}
	if !strings.Contains(string(got), `xmlns="`+XMLNS+`"`) {
		t.Errorf("expected the 0.9 namespace by default:\n%s", got)
	}
}

// IndexXML is the no-filesystem counterpart of CreateSitemapIndex, for callers
// that upload the bytes instead of writing a file.
func TestIndexXMLReturnsBytesWithoutTouchingDisk(t *testing.T) {
	got, err := IndexXML(Index{Sitemaps: []Sitemap{{Loc: "https://ex.com/s_1.xml.gz"}}})
	if err != nil {
		t.Fatalf("IndexXML: %v", err)
	}
	if !strings.HasPrefix(string(got), PREAMBLE) {
		t.Errorf("missing xml preamble:\n%s", got)
	}
	if !strings.Contains(string(got), "<loc>https://ex.com/s_1.xml.gz</loc>") {
		t.Errorf("sitemap entry missing:\n%s", got)
	}
	// raw, not gzipped - callers compress if their destination needs it
	if len(got) > 1 && got[0] == 0x1f && got[1] == 0x8b {
		t.Error("IndexXML returned gzipped bytes, want raw xml")
	}
}

func TestSchemaValidationEmitsSchemaLocation(t *testing.T) {
	got, err := createSitemapXml(URLSet{URLs: []URL{
		{Loc: "https://ex.com/p", Alternates: []Alternate{{HrefLang: "es", Href: "https://ex.com/es/p"}}},
	}}, false, true)
	if err != nil {
		t.Fatalf("createSitemapXml: %v", err)
	}
	s := string(got)
	for _, want := range []string{
		`xmlns:xsi="` + XSI + `"`,
		`xsi:schemaLocation="` + SCHEMALOCATION + `"`,
		`xmlns:xhtml="` + XHTMLSCHEMA + `"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in:\n%s", want, s)
		}
	}
	// the schemaLocation mapping requires the xhtml11 uri, not the usual one
	if strings.Contains(s, `xmlns:xhtml="`+XMLNSXHTML+`"`) {
		t.Errorf("validation mode should not use the production xhtml uri:\n%s", s)
	}
}

// Off by default: production output must not carry the validation attributes.
func TestSchemaValidationOffByDefault(t *testing.T) {
	g := NewSitemapGroup("prod", false)
	g.Add(URL{Loc: "https://ex.com/p", Alternates: []Alternate{{HrefLang: "es", Href: "https://ex.com/es/p"}}})
	for f := range g.Files() {
		s := string(f.Content)
		if strings.Contains(s, "xsi:schemaLocation") || strings.Contains(s, XHTMLSCHEMA) {
			t.Errorf("validation attributes leaked into default output:\n%s", s)
		}
		if !strings.Contains(s, `xmlns:xhtml="`+XMLNSXHTML+`"`) {
			t.Errorf("expected the production xhtml uri:\n%s", s)
		}
	}
}

func TestSetSchemaValidationOnGroup(t *testing.T) {
	g := NewSitemapGroup("dev", false)
	g.SetSchemaValidation(true)
	g.Add(URL{Loc: "https://ex.com/p"})
	seen := 0
	for f := range g.Files() {
		seen++
		if !strings.Contains(string(f.Content), "xsi:schemaLocation") {
			t.Errorf("group did not propagate schema validation:\n%s", f.Content)
		}
	}
	if seen != 1 {
		t.Errorf("got %d files, want 1", seen)
	}
}
