sitemap
=======
script to generate sitemaps with multiple files of same group


##Usage


```
package main

import (
    "github.com/vitalbh/sitemap"
    "sync"
)
var wg sync.WaitGroup
func main(){

    group := sitemap.NewSitemapGroup("./","sitemap_group1")

    wg.Add(10000)
    go func(){
            for i := 0; i < 10000; i++ {
                group.Add(sitemap.URL{Loc: "http://example.com/"})
                wg.Done()
            }
    }()
    wg.Add(250000)
    go func(){

            for i := 0; i < 250000; i++ {
                group.Add(sitemap.URL{Loc: "http://example2.com/"})
                wg.Done()
            }
    }()

    group2 := sitemap.NewSitemapGroup("./","sitemap_group2")

    wg.Add(10000)
    go func(){
            for i := 0; i < 10000; i++ {
                group2.Add(sitemap.URL{Loc: "http://example.com/blog/"})
                wg.Done()
            }
    }()

    wg.Wait()
    group.CloseGroup()
    group2.CloseGroup()
    sitemap.CreateSitemapIndex("./index.xml.gz","./", "http://domain.com.br/")
}
```
