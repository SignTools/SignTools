package assets

import "time"

type ServerFile struct {
	Id          string
	IsSigned    bool
	Name        string
	ModTime     time.Time
	WorkflowUrl string
	ManifestUrl string
	DownloadUrl string
	DeleteUrl   string
}

type IndexData struct {
	Files []ServerFile
}

type ManifestData struct {
	DownloadUrl string
	BundleId    string
	Title       string
}
