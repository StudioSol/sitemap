package main

import (
	"log"
	"os"
	"sync"

	"github.com/StudioSol/sitemap"
)

var wg sync.WaitGroup

func main() {
	group := sitemap.NewSitemapGroup("user", false)
	group2 := sitemap.NewSitemapGroup("blog", false)

	for i := 0; i < 300000; i++ {
		group.Add(sitemap.URL{Loc: "http://example.com/"})
	}
	for i := 0; i < 25; i++ {
		group.Add(sitemap.URL{Loc: "http://example2.com/"})
	}
	for i := 0; i < 10; i++ {
		group2.Add(sitemap.URL{Loc: "http://example.com/blog/"})
	}
	for i := 0; i < 10000; i++ {
		group2.Add(sitemap.URL{Loc: "http://example.com/blog/"})
	}

	files1 := group.Files()
	files2 := group2.Files()

	for file := range files1 {
		log.Println(file.Name)

		f, err := os.Create(file.Name)
		if err != nil {
			log.Fatal(err)
		}

		err = file.Write(f)
		if err != nil {
			log.Fatal(err)
		}
	}

	for file := range files2 {
		log.Println(file.Name)

		f, err := os.Create(file.Name)
		if err != nil {
			log.Fatal(err)
		}

		err = file.Write(f)
		if err != nil {
			log.Fatal(err)
		}
	}

	//generate index - by last execution paths
	URLs := append(group.URLs(), group2.URLs()...)
	index := sitemap.CreateIndexBySlice(URLs, "http://domain.com.br/")

	log.Println("creating index...")
	err := sitemap.CreateSitemapIndex("index.xml.gz", index)
	if err != nil {
		log.Fatal(err)
	}

	sitemap.PingSearchEngines("http://domain.com.br/index.xml.gz")
}
