package assets

import _ "embed"

//go:embed index.gohtml
var IndexHtml string

//go:embed manifest.xml
var ManifestHtml string
