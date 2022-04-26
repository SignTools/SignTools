package assets

import (
	"embed"
	_ "embed"
	"io/ioutil"
)

//go:embed index.gohtml
var IndexHtml string

//go:embed install.gohtml
var InstallHtml string

//go:embed 2fa.html
var TwoFactorHtml string

//go:embed rename.gohtml
var RenameHtml string

//go:embed manifest.xml
var ManifestPlist string

//go:embed favicon.png
var favIconFS embed.FS

//go:embed certs
var AppleCerts embed.FS

var FavIconFile, _ = favIconFS.Open("favicon.png")
var FavIconBytes, _ = ioutil.ReadAll(FavIconFile)
var FavIconStat, _ = FavIconFile.Stat()
