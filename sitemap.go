package sitemap

import (
    "log"
    "strconv"
    "strings"
    "io/ioutil"
    "compress/gzip"
    "os"
    "time"
)

type sitemapGroup struct {
	name    string
    urls    []URL
    url_channel    chan URL
    group_count    int
    folder  string
}

func (s *sitemapGroup) Add (url URL ) {
    s.url_channel <- url
}

func (s *sitemapGroup) CloseGroup ( ) {
    s.Create(s.getURLSet())
    close(s.url_channel)
    s.Clear()
}

func (s *sitemapGroup) Clear ( ) {
    s.urls = []URL{}
}

func (s *sitemapGroup) getURLSet() URLSet{
    return URLSet{URLs: s.urls}
}

func (s *sitemapGroup) Create (url_set URLSet) {

        xml := createXML(url_set)

        var path string = s.folder + s.name + "_" + strconv.Itoa(s.group_count) + ".xml.gz"

        err := saveXml(xml, path)

        if err != nil {
            log.Fatal("File not saved:", err)
        }
        log.Printf("Sitemap created on %s", path)
        s.group_count++

}

func NewSitemapGroup(folder string,name string) *sitemapGroup {
	s := new(sitemapGroup)
    s.name = strings.Replace(name, ".xml.gz", "", 1)
    s.group_count = 1
    s.url_channel = make(chan URL)
    _, err := ioutil.ReadDir(folder)
	if err != nil {
		log.Fatal("Dir not allowed - ", err)
	}
    s.folder = folder

    go func() {
        for entry := range s.url_channel {

            s.urls = append(s.urls, entry)

            if len(s.urls) == MAXURLSETSIZE {

                go func(urls URLSet) {
                    s.Create(urls)
                }(s.getURLSet())

                s.Clear()
            }
        }
    }()

	return s
}


func createXML(group URLSet) (sitemapXml []byte) {
    sitemapXml, err := createSitemapXml(group)
    if err != nil {
        log.Fatal("work failed:", err)
    }
    return
}

func saveXml(xmlFile []byte, path string) (err error){

        fo, err := os.Create(path)
        if err != nil {
            return err
        }
        defer fo.Close()

        zip := gzip.NewWriter(fo)
        defer zip.Close()
        _, err = zip.Write(xmlFile)
        if err != nil {
            return err
        }

        return err

}

func CreateSitemapIndex(indexFile string, folder string, public_dir string) (err error) {


    fs, err := ioutil.ReadDir(folder)
    if err != nil {
        return err
    }

    var index = Index{Sitemaps:[]Sitemap{}}
    //search sitemaps
    for _, f := range fs {
        if strings.HasSuffix(f.Name(), ".xml.gz") && !strings.HasSuffix(indexFile, f.Name()) {
            index.Sitemaps = append(index.Sitemaps, Sitemap{Loc: public_dir + f.Name(),LastMod: time.Now()})
        }
    }

    //create xml
    indexXml, err := createSitemapIndexXml(index)
    if err != nil {
        return err
    }
    //touch path
    fo, err := os.Create(indexFile)
    if err != nil {
        return err
    }
    defer fo.Close()
    //Save gzip
    zip := gzip.NewWriter(fo)
    defer zip.Close()
    _, err = zip.Write(indexXml)
    if err != nil {
        return err
    }

    log.Printf("Sitemap Index created on %s", indexFile)
    return err
}
