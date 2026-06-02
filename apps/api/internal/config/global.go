package config

var globalConfig App

// Init stores the config globally for access from anywhere
func Init(cfg App) {
	globalConfig = cfg
}

// Get returns the global config instance
func Get() App {
	return globalConfig
}
