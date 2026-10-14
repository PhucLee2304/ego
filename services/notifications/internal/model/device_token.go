package model

type DeviceToken struct {
	ID       uint          `gorm:"primaryKey"`
	UserID   string        `gorm:"type:text;not null;index"`
	DeviceID string        `gorm:"type:text;not null;uniqueIndex"`
	Token    string        `gorm:"type:text;not null;uniqueIndex"`
	Platform TokenPlatform `gorm:"type:text;not null"`
}

type TokenPlatform string

const (
	TokenPlatformWeb     TokenPlatform = "WEB"
	TokenPlatformAndroid TokenPlatform = "ANDROID"
	TokenPlatformIOS     TokenPlatform = "IOS"
)
