package assets

type App struct {
	Id           string
	Status       int
	Name         string
	ModTime      string
	WorkflowUrl  string
	ManifestUrl  string
	DownloadUrl  string
	TwoFactorUrl string
	RestartUrl   string
	DeleteUrl    string
	RenameUrl    string
	ProfileName  string
	BundleId     string
}

const (
	AppStatusProcessing = 0
	AppStatusSigned     = 1
	AppStatusFailed     = 2
	AppStatusWaiting    = 3
)

type Profile struct {
	Id        string
	Name      string
	IsAccount bool
}

type Builder struct {
	Id   string
	Name string
}

type FormNames struct {
	FormFile            string
	FormFileId          string
	FormProfileId       string
	FormBuilderId       string
	FormAppDebug        string
	FormAllDevices      string
	FormFileShare       string
	FormToken           string
	FormId              string
	FormIdOriginal      string
	FormIdProv          string
	FormIdCustom        string
	FormIdCustomText    string
	FormIdEncode        string
	FormIdForceOriginal string
}

type IndexData struct {
	Apps     []App
	Profiles []Profile
	Builders []Builder
	FormNames
}

type ManifestData struct {
	DownloadUrl string
	BundleId    string
	Title       string
}

type RenameData struct {
	AppName string
}
