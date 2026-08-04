package sitemap

import (
	"compress/gzip"
	"encoding/xml"
	"errors"
	"os"
	"time"
)

const (
	XMLNS         = "http://www.sitemaps.org/schemas/sitemap/0.9"
	XMLNSMOBILE   = "http://www.google.com/schemas/sitemap-mobile/1.0"
	XMLNSXHTML    = "http://www.w3.org/1999/xhtml"
	PREAMBLE      = "<?xml version=\"1.0\" encoding=\"UTF-8\"?>"
	MAXURLSETSIZE = 50000
	MAXFILESIZE   = 10 * 1024 * 1024 //10mb

	// XDefault is the hreflang value that marks the fallback version served
	// when no other localized version matches the user's language.
	XDefault = "x-default"
)

var (
	ErrMaxUrlSetSize = errors.New("exceeded maximum number of URLs allowed in sitemap")
	ErrMaxFileSize   = errors.New("exceeded maximum file size of a sitemap (10mb)")
	ISMOBILE         = new(struct{})
)

type ChangeFreq string

const (
	Always  ChangeFreq = "always"
	Hourly  ChangeFreq = "hourly"
	Daily   ChangeFreq = "daily"
	Weekly  ChangeFreq = "weekly"
	Monthly ChangeFreq = "monthly"
	Yearly  ChangeFreq = "yearly"
	Never   ChangeFreq = "never"
)

// Alternate is a localized version of a URL, rendered as an
// <xhtml:link rel="alternate"> annotation.
//
// See https://developers.google.com/search/docs/specialty/international/localized-versions#sitemap
type Alternate struct {
	// HrefLang is the language (and optional region) of the alternate, in
	// ISO 639-1 / ISO 3166-1 Alpha 2 form: "pt", "pt-br", "en-us". The
	// special value XDefault marks the language-agnostic fallback.
	HrefLang string
	// Href is the fully-qualified URL of the localized version.
	Href string
}

// MarshalXML renders the alternate as <xhtml:link rel="alternate" hreflang=".."
// href=".."/>. rel is fixed by the spec, so it is written here rather than
// exposed as a field that callers could leave empty.
func (a Alternate) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = xml.Name{Local: "xhtml:link"}
	start.Attr = []xml.Attr{
		{Name: xml.Name{Local: "rel"}, Value: "alternate"},
		{Name: xml.Name{Local: "hreflang"}, Value: a.HrefLang},
		{Name: xml.Name{Local: "href"}, Value: a.Href},
	}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	return e.EncodeToken(start.End())
}

type URL struct {
	Loc string `xml:"loc"`
	// Alternates lists every localized version of this URL. Google requires
	// the set to be complete and self-referential: each localized page must
	// list all versions of itself, including its own Loc.
	Alternates []Alternate `xml:"xhtml:link,omitempty"`
	LastMod    *time.Time  `xml:"lastmod,omitempty"`
	ChangeFreq ChangeFreq  `xml:"changefreq,omitempty"`
	Priority   float64     `xml:"priority,omitempty"`
	Mobile     *struct{}   `xml:"mobile:mobile,omitempty"`
}

type URLSet struct {
	XMLName     xml.Name `xml:"urlset"`
	XMLNS       string   `xml:"xmlns,attr"`
	XMLNSMOBILE string   `xml:"xmlns:mobile,attr,omitempty"`
	XMLNSXHTML  string   `xml:"xmlns:xhtml,attr,omitempty"`
	URLs        []URL    `xml:"url"`
}

type Index struct {
	XMLName  xml.Name  `xml:"sitemapindex"`
	XMLNS    string    `xml:"xmlns,attr"`
	Sitemaps []Sitemap `xml:"sitemap"`
}

type Sitemap struct {
	Loc     string     `xml:"loc"`
	LastMod *time.Time `xml:"lastmod,omitempty"`
}

func createSitemapXml(urlset URLSet, isMobile bool) (sitemapXML []byte, err error) {
	if len(urlset.URLs) > MAXURLSETSIZE {
		err = ErrMaxUrlSetSize
		return
	}
	urlset.XMLNS = XMLNS

	if isMobile {
		urlset.XMLNSMOBILE = XMLNSMOBILE
	}

	// The xhtml namespace is declared only when some URL actually carries
	// alternates, so sitemaps without them stay byte-identical.
	if hasAlternates(urlset.URLs) {
		urlset.XMLNSXHTML = XMLNSXHTML
	}

	sitemapXML = []byte(PREAMBLE)
	var urlsetXML []byte
	urlsetXML, err = xml.Marshal(urlset)
	if err == nil {
		sitemapXML = append(sitemapXML, urlsetXML...)
	}
	if len(sitemapXML) > MAXFILESIZE {
		err = ErrMaxFileSize
		return
	}
	return
}

func hasAlternates(urls []URL) bool {
	for _, u := range urls {
		if len(u.Alternates) > 0 {
			return true
		}
	}
	return false
}

func createSitemapIndexXml(index Index) (indexXML []byte, err error) {
	if len(index.Sitemaps) > MAXURLSETSIZE {
		err = ErrMaxUrlSetSize
		return
	}
	index.XMLNS = XMLNS
	indexXML = []byte(PREAMBLE)
	var sitemapIndexXML []byte
	sitemapIndexXML, err = xml.Marshal(index)
	if err == nil {
		indexXML = append(indexXML, sitemapIndexXML...)
	}
	if len(indexXML) > MAXFILESIZE {
		return nil, ErrMaxFileSize
	}
	return
}

// Save and gzip xml
func saveXml(xmlFile []byte, path string) (err error) {
	fo, err := os.Create(path)
	defer fo.Close()

	if err != nil {
		return err
	}

	zip, _ := gzip.NewWriterLevel(fo, gzip.BestCompression)
	defer zip.Close()

	_, err = zip.Write(xmlFile)
	if err != nil {
		return err
	}

	return err

}
