package assets

import "time"

type ServerFile struct {
	Id           string
	IsSigned     bool
	Name         string
	UploadedTime time.Time
	JobUrl       string
	ManifestUrl  string
	DownloadUrl  string
}

type IndexData struct {
	Files []ServerFile
}

type ManifestData struct {
	DownloadUrl string
	BundleId    string
	Title       string
}
