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
	PREAMBLE      = "<?xml version=\"1.0\" encoding=\"UTF-8\"?>"
	MAXURLSETSIZE = 5e4
	MAXFILESIZE   = 10 * 1024 * 1024
)

var (
	ErrMaxUrlSetSize = errors.New("exceeded maximum number of URLs allowed in sitemap")
	ErrMaxFileSize   = errors.New("exceeded maximum file size of a sitemap (10mb)")
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
	LastMod    *time.Time `xml:"lastmod,omitempty"`
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
	LastMod *time.Time `xml:"lastmod,omitempty"`
}

func createSitemapXml(urlset URLSet) (sitemapXML []byte, err error) {
	if len(urlset.URLs) > MAXURLSETSIZE {
		err = ErrMaxUrlSetSize
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
		err = ErrMaxFileSize
		return
	}
	return
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
		err = ErrMaxFileSize
		return
	}
	return
}

//Save and gzip xml
func saveXml(xmlFile []byte, path string) (err error) {

	fo, err := os.Create(path)
	if err != nil {
		return err
	}
	defer fo.Close()

	zip, _ := gzip.NewWriterLevel(fo, gzip.BestCompression)
	defer zip.Close()
	_, err = zip.Write(xmlFile)
	if err != nil {
		return err
	}

	return err

}
