package assets

import (
	"embed"
	_ "embed"
	"io/ioutil"
)

//go:embed index.gohtml
var IndexHtml string

//go:embed manifest.xml
var ManifestPlist string

//go:embed favicon.png
var favIconFS embed.FS

var FavIconFile, _ = favIconFS.Open("favicon.png")
var FavIconBytes, _ = ioutil.ReadAll(FavIconFile)
var FavIconStat, _ = FavIconFile.Stat()
