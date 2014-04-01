package sitemap

import (
  "time"
  "errors"
  "encoding/xml"
)

const (
  XMLNS = "http://www.sitemaps.org/schemas/sitemap/0.9"
  PREAMBLE = "<?xml version=\"1.0\" encoding=\"UTF-8\"?>"
  MAXURLSETSIZE = 5e4
  MAXFILESIZE = 10 * 1024 * 1024
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

type URL struct {
	Loc        string     `xml:"loc"`
	LastMod    time.Time  `xml:"lastmod,omitempty"`
	ChangeFreq ChangeFreq `xml:"changefreq,omitempty"`
	Priority   float64    `xml:"priority,omitempty"`
}

type URLSet struct {
	XMLName xml.Name `xml:"urlset"`
	XMLNS   string   `xml:"xmlns,attr"`
	URLs    []URL    `xml:"url"`
}

type Index struct {
	XMLName  xml.Name  `xml:"sitemapindex"`
	XMLNS    string    `xml:"xmlns,attr"`
	Sitemaps []Sitemap `xml:"sitemap"`
}

type Sitemap struct {
	Loc     string     `xml:"loc"`
	LastMod time.Time `xml:"lastmod,omitempty"`
}


func createSitemapXml(urlset URLSet) (sitemapXML []byte, err error) {
	if len(urlset.URLs) > MAXURLSETSIZE {
		err = errors.New("exceeded maximum number of URLs allowed in sitemap")
		return
	}
	urlset.XMLNS = XMLNS
	sitemapXML = []byte(PREAMBLE)
	var urlsetXML []byte
	urlsetXML, err = xml.Marshal(urlset)
	if err == nil {
		sitemapXML = append(sitemapXML, urlsetXML...)
	}
	if len(sitemapXML) > MAXFILESIZE {
        err = errors.New("exceeded maximum file size of a sitemap")
        return
	}
	return
}

func createSitemapIndexXml(index Index) (indexXML []byte, err error) {
	if len(index.Sitemaps) > MAXURLSETSIZE {
		err = errors.New("exceeded maximum number of URLs allowed in sitemap")
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
		err = errors.New("exceeded maximum file size of a sitemap")
        return
	}
	return
}
