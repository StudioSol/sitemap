# sitemap [![GoDoc](https://godoc.org/github.com/StudioSol/sitemap?status.png)](https://godoc.org/github.com/StudioSol/sitemap)

Generates sitemaps and index files based on the sitemaps.org protocol.


### sitemap.NewSitemapGroup(dir string,name string)

Creates a new group of sitemaps that used a common name.

~~~ go
group := sitemap.NewSitemapGroup("/var/www/blog/public/sitemaps/","blog")
~~~

If the sitemap exceed the limit of 50k urls, new sitemaps will have a numeric suffix to the name. Example:
- blog_1.xml.gz
- blog_2.xml.gz

#### group.Add(url sitemap.URL)

Add sitemap.URL to group

~~~ go
now := time.Now()
group.Add(sitemap.URL{
    Loc: "http://example.com/blog/1/",
    ChangeFreq: sitemap.Hourly,
    LastMod: &now,
    Priority: 0.9
    })
~~~

#### Localized versions (hreflang alternates)

To annotate a URL with its localized versions, fill `Alternates`. Each entry becomes an
`<xhtml:link rel="alternate" hreflang=".." href=".."/>` inside the `<url>`, per
[Google's sitemap spec for localized versions](https://developers.google.com/search/docs/specialty/international/localized-versions#sitemap).

~~~ go
group.Add(sitemap.URL{
    Loc: "https://example.com/en/page",
    Alternates: []sitemap.Alternate{
        {HrefLang: "en", Href: "https://example.com/en/page"},
        {HrefLang: "pt-br", Href: "https://example.com/pt/page"},
        {HrefLang: sitemap.XDefault, Href: "https://example.com/page"},
    },
})
~~~

Produces:

~~~ xml
<url>
  <loc>https://example.com/en/page</loc>
  <xhtml:link rel="alternate" hreflang="en" href="https://example.com/en/page"></xhtml:link>
  <xhtml:link rel="alternate" hreflang="pt-br" href="https://example.com/pt/page"></xhtml:link>
  <xhtml:link rel="alternate" hreflang="x-default" href="https://example.com/page"></xhtml:link>
</url>
~~~

`rel="alternate"` is written for you, and the `xmlns:xhtml` declaration is added to the
`<urlset>` only when at least one URL in the file carries alternates — so sitemaps without
them stay byte-identical to before.

Two rules come from Google, not from this library, and are the caller's responsibility:

- **The set must be self-referential.** Every localized page must list *all* versions of
  itself, including its own `Loc`. In the example above, the `pt-br` page has to carry the
  same three `Alternates`.
- **`hreflang` must be a valid ISO 639-1 language**, optionally with an ISO 3166-1 Alpha 2
  region (`pt`, `pt-br`, `en-us`), or `sitemap.XDefault` for the fallback version.

Alternates inflate each `<url>` by roughly 100 bytes per language, so a sitemap with many
locales reaches the 10MB file limit well before the 50k URL limit. The group splits on
either limit automatically.


#### group.Close()

Handle the rest of the url that has not been added to any sitemap and add, in addition to clean variables and close the channel group

#### sitemap.CloseGroups(groups ...*SitemapGroup) (done <-chan bool)

if you use several groups of sitemap is safer use this function to close all groups for you before creating the index. Returns a channel with the done signal.

~~~ go
	//release after close all groups
	<-sitemap.CloseGroups(group, group2)

	//generate index - by last execution paths
	savedSitemaps := group.GetSavedSitemaps()
	sitemapsgroup2 := group.GetSavedSitemaps()
	savedSitemaps = append(savedSitemaps, sitemapsgroup2...)

~~~

### Error handling on index groups

`IndexGroup.Create` returns an `error`. Before v1.1.0 it called `log.Fatal`, terminating the
host process — a library has no business doing that.

`Create` is also called internally by the goroutine that drains `Add`, where there is no
caller to return to. Those failures are logged and recorded on the group, and `Err()`
reports the first one:

~~~ go
<-group.Close()
if err := group.Err(); err != nil {
    return err
}
~~~

Errors wrap their cause, so `errors.Is` works as expected:

~~~ go
if errors.Is(err, sitemap.ErrMaxFileSize) { /* ... */ }
~~~

### Creating the index file

Currently we have 2 ways to create the index, searching for files in the directory or passing a slice of urls to sitemaps. To generate the slice of sitemaps generated in the last run we GetSavedSitemaps function.

#### group.GetSavedSitemaps() []string

Returns an array of urls in the sitemaps created script execution

~~~ go
savedSitemaps := group.GetSavedSitemaps()
~~~

#### sitemap.CreateIndexBySlice(savedSitemaps, path) sitemap.Index

~~~ go
index := sitemap.CreateIndexBySlice(savedSitemaps, "http://example.com.br/sitemaps/")
~~~

#####OR

#### sitemap.CreateIndexByScanDir(sitemaps_dir,index_path, path) sitemap.Index

Search all the xml.gz sitemaps_dir directory, uses the modified date of the file as lastModified

path_index is included for the function does not include the url of the index in your own content, if it is present in the same directory.

~~~ go
index := sitemap.CreateIndexByScanDir("/var/www/blog/public/sitemaps/", "/var/www/blog/public/index.xml.gz", "http://example.com.br/sitemaps/")
~~~
__Warning__: this release do not control old sitemaps, when using this method the index can be created with sitemaps that are no longer used. In case you need to delete manually.


#### sitemap.CreateSitemapIndex(path, sitemap.Index)

creates and gzip the xml index

~~~ go
sitemap.CreateSitemapIndex("/var/www/blog/public/index.xml.gz", index)
~~~

### sitemap.PingSearchEngines(public_index_path)
Sends a ping to search engines indicating that the index has been updated.

Currently supports Google and Bing.
~~~ go
sitemap.PingSearchEngines("http://exemple.com/index.xml.gz")
~~~



## Example
There is a very simple example of using the example folder.
