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
