//go:build ignore

package main

import (
	"log"
	"os"

	"github.com/StudioSol/sitemap"
)

type locale struct {
	lang   string
	prefix string
}

// locales served for every page. An ordered slice, not a map: map iteration
// order is randomized in Go, which would reshuffle the alternates on every run
// and produce a needless diff in the generated sitemaps.
var locales = []locale{
	{"pt-br", "https://example.com/pt/"},
	{"en", "https://example.com/en/"},
	{"es", "https://example.com/es/"},
}

// alternatesFor builds the complete, self-referential alternate set for a page.
// Google requires every localized version to list all versions of itself,
// including its own Loc - so the same slice is attached to each of them.
func alternatesFor(slug string) []sitemap.Alternate {
	alts := make([]sitemap.Alternate, 0, len(locales)+1)
	for _, l := range locales {
		alts = append(alts, sitemap.Alternate{HrefLang: l.lang, Href: l.prefix + slug})
	}
	// fallback for users whose language matches none of the above
	alts = append(alts, sitemap.Alternate{HrefLang: sitemap.XDefault, Href: "https://example.com/" + slug})
	return alts
}

func main() {
	group := sitemap.NewSitemapGroup("i18n", false)

	for _, slug := range []string{"about", "contact", "pricing"} {
		alts := alternatesFor(slug)
		// one <url> entry per localized version, each carrying the full set
		for _, l := range locales {
			group.Add(sitemap.URL{
				Loc:        l.prefix + slug,
				Alternates: alts,
				ChangeFreq: sitemap.Weekly,
				Priority:   0.8,
			})
		}
	}

	for file := range group.Files() {
		log.Println(file.Name)

		f, err := os.Create(file.Name)
		if err != nil {
			log.Fatal(err)
		}
		if err := file.Write(f); err != nil {
			f.Close()
			log.Fatal(err)
		}
		f.Close()
	}
}
