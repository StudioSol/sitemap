//Generates sitemaps and index files based on the sitemaps.org protocol.
//facilitates the creation of sitemaps for large amounts of urls.
// For a full guide visit https://github.com/StudioSol/Sitemap
package sitemap

import (
	"io/ioutil"
	"log"
	"strings"
	"time"
)

//Creates a new group of sitemaps that used a common name.
//If the sitemap exceed the limit of 50k urls, new sitemaps will have a numeric suffix to the name. Example:
//- blog_1.xml.gz
//- blog_2.xml.gz
func NewSitemapGroup(folder string, name string) (*SitemapGroup, error) {
	s := new(SitemapGroup)
	err := s.Configure(name, folder)
	if err != nil {
		return s, err
	}
	go s.Initialize()
	return s, nil
}

//Creates a new group of sitemaps indice that used a common name.
//If the sitemap exceed the limit of 50k urls, new sitemaps will have a numeric suffix to the name. Example:
//- blog_1.xml.gz
//- blog_2.xml.gz
func NewIndexGroup(folder string, name string) (*IndexGroup, error) {
	s := new(IndexGroup)
	err := s.Configure(name, folder)
	if err != nil {
		return s, err
	}
	go s.Initialize()
	return s, nil
}

//Search all the xml.gz sitemaps_dir directory, uses the modified date of the file as lastModified
//path_index is included for the function does not include the url of the index in your own content, if it is present in the same directory.
func CreateIndexByScanDir(targetDir string, indexFileName string, public_url string) (index Index) {

	index = Index{Sitemaps: []Sitemap{}}

	fs, err := ioutil.ReadDir(targetDir)
	if err != nil {
		return
	}

	for _, f := range fs {
		if strings.HasSuffix(f.Name(), ".xml.gz") && !strings.HasSuffix(indexFileName, f.Name()) {
			lastModified := f.ModTime()
			index.Sitemaps = append(index.Sitemaps, Sitemap{Loc: public_url + f.Name(), LastMod: &lastModified})
		}
	}
	return
}

//Returns an index sitemap starting from a slice of urls
func CreateIndexBySlice(urls []string, public_url string) (index Index) {

	index = Index{Sitemaps: []Sitemap{}}

	if len(urls) > 0 {
		for _, fileName := range urls {
			lastModified := time.Now()
			index.Sitemaps = append(index.Sitemaps, Sitemap{Loc: public_url + fileName, LastMod: &lastModified})
		}
	}

	return
}

//Creates and gzip the xml index
func CreateSitemapIndex(indexFilePath string, index Index) (err error) {

	//create xml
	indexXml, err := createSitemapIndexXml(index)
	if err != nil {
		return err
	}
	err = saveXml(indexXml, indexFilePath)
	log.Printf("Sitemap Index created on %s", indexFilePath)
	return err
}
