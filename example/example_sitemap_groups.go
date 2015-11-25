package main

import (
	"sync"

	"github.com/StudioSol/sitemap"
)

var wg sync.WaitGroup

func main() {

	group := sitemap.NewSitemapGroup("./", "sitemap_group1")
	group2 := sitemap.NewSitemapGroup("./", "sitemap_group2")

	wg.Add(10000)
	go func() {
		for i := 0; i < 10000; i++ {
			group.Add(sitemap.URL{Loc: "http://example.com/"})
			wg.Done()
		}
	}()

	wg.Add(250000)
	go func() {

		for i := 0; i < 250000; i++ {
			group.Add(sitemap.URL{Loc: "http://example2.com/"})
			wg.Done()
		}
	}()

	wg.Add(10000)
	go func() {
		for i := 0; i < 10000; i++ {
			group2.Add(sitemap.URL{Loc: "http://example.com/blog/"})
			wg.Done()
		}
	}()

	wg.Add(30000)
	go func() {
		for i := 0; i < 30000; i++ {
			group2.Add(sitemap.URL{Loc: "http://example.com/blog/"})
			wg.Done()
		}
	}()

	wg.Wait()

	//release after close all groups
	<-sitemap.CloseGroups(group, group2)

	//generate index - by scanning the folder (WARNING)
	//index := sitemap.CreateIndexByScanDir("./", "./index.xml.gz", "http://domain.com.br/")

	//generate index - by last execution paths
	savedSitemaps := sitemap.GetSavedSitemaps()
	index := sitemap.CreateIndexBySlice(savedSitemaps, "http://domain.com.br/")

	sitemap.CreateSitemapIndex("./index.xml.gz", index)

	sitemap.PingSearchEngines("http://domain.com.br/index.xml.gz")
}
