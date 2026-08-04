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
	}}, false)
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
	}}, false)
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
	}}, false)
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
	got, err := createSitemapXml(URLSet{URLs: []URL{{Loc: "https://ex.com/p"}}}, false)
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
	got, err := createSitemapXml(URLSet{URLs: []URL{{Loc: "https://ex.com/p", Mobile: ISMOBILE}}}, true)
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
	}}, true)
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
	}}, false)
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
