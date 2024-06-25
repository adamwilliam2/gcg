package dao

import (
	"time"
)

type AppVersionDomain struct {
	AppID         string    `json:"app_id" db:"app_id"`
	AppVersion    string    `json:"app_version" db:"app_version"`
	AppMinVersion string    `json:"app_min_version" db:"app_min_version"`
	DownloadURL   *string   `json:"download_url" db:"download_url"`
	CreatedAt     time.Time `json:"created_at" db:"created_at" binding:"-"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at" binding:"-"`
}

func (AppVersionDomain) TableName() string {
	return "app_version"
}
