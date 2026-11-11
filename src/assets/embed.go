package assets

import (
	"embed"
	_ "embed"
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

//go:embed js vendor favicon.png
var StaticFS embed.FS

//go:embed certs
var AppleCerts embed.FS
