package assets

import "time"

type App struct {
	Id          string
	IsSigned    bool
	Name        string
	ModTime     time.Time
	WorkflowUrl string
	ManifestUrl string
	DownloadUrl string
	DeleteUrl   string
	ProfileName string
}

type Profile struct {
	Id   string
	Name string
}

type IndexData struct {
	Apps            []App
	Profiles        []Profile
	FormFile        string
	FormProfileName string
	FormAppDebug    string
	FormAllDevices  string
	FormFileShare   string
	FormAlignAppId  string
}

type ManifestData struct {
	DownloadUrl string
	BundleId    string
	Title       string
}
