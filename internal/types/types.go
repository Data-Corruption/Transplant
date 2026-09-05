package types

import (
	"time"

	"github.com/Data-Corruption/Transplant/internal/build"
)

type Configuration struct {
	LogLevel string `json:"logLevel"`

	UpdateNotifications    bool      `json:"updateNotifications"`
	LastUpdateCheck        time.Time `json:"lastUpdateCheck"`
	BackgroundUpdateChecks bool      `json:"backgroundUpdateChecks"`
	UpdateCheckSource      string    `json:"updateCheckSource"`
	LatestUpdateVersion    string    `json:"latestUpdateVersion"`

	// LastShutdownVersion is compared with the running version after a restart
	// to infer whether an update occurred.
	LastShutdownVersion string `json:"lastShutdownVersion"`
}

func DefaultConfig(buildInfo build.BuildInfo) Configuration {

	return Configuration{
		LogLevel:               buildInfo.DefaultLogLevel,
		UpdateNotifications:    true,
		BackgroundUpdateChecks: true,
		LastUpdateCheck:        time.Time{},
	}
}
