type addonInstallOptions struct {
	values           []string
	version          string
	valuesFiles      []string
	set              map[string]string
	all              bool
	nonDefault       bool
	dryRun           bool
	defaultVersion   bool
	skipDependencies bool // Flag to skip installing dependencies automatically
	// used to install all addons use defaultConfig after initialization
	initDefaultConfig string
}
