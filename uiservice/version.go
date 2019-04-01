package uiservice

type UIVersion string

var (
	PreBundledUIVersion = UIVersion("")
)

type VersionChangeListener func(UIVersion)

type VersionStore interface {
	CurrentVersion() (UIVersion, error)
	UpdateCurrentVersion(UIVersion) error
	WatchForVersionChange(VersionChangeListener) error
}
