package models

import (
	"errors"
)

type Filesystem struct {
	Fs          string         `json:"fs"`
	Permissions []string       `json:"permissions"`
	Params      map[string]any `json:"params"`
}

type User struct {
	ID                int64         `json:"id"`
	Username          string        `json:"username"`
	Password          string        `json:"password"`
	Filesystems       []*Filesystem `json:"filesystems"`
	DefaultFilesystem string        `json:"default_filesystem"`
	Filesystem        *Filesystem   `json:"filesystem"`
	Permissions       []string      `json:"permissions"`
}

func (u User) GetFilesystem() (*Filesystem, error) {
	if len(u.Filesystems) == 0 {
		return nil, nil
	}
	if u.DefaultFilesystem != "" {
		for _, fs := range u.Filesystems {
			if u.DefaultFilesystem == fs.Fs {
				u.Filesystem = fs
				break
			}
		}
		if u.Filesystem == nil {
			return nil, errors.New("no filesystem for user")
		}
	}
	if u.Filesystem == nil {
		u.Filesystem = u.Filesystems[0]
	}
	return u.Filesystem, nil
}

// TypeCredential is the type of credential used for authentication.
type TypeCredential string

const (
	Password  TypeCredential = "PASSWORD"
	TwoFactor TypeCredential = "TWO_FACTOR"
	APIKey    TypeCredential = "API_Key"
	Oauth     TypeCredential = "OAUTH"
)

type Integration string

const (
	SFTP Integration = "SFTP"
	Mail Integration = "MAIL"
)

// TypeProvider is the type of provider used for authentication.
type TypeProvider string

const (
	Local   TypeProvider = "LOCAL"
	Cognito TypeProvider = "COGNITO"
	Dropbox TypeProvider = "DROPBOX"
	GDrive  TypeProvider = "GDRIVE"
	S3      TypeProvider = "S3"
)

type Credential struct {
	Credential     string         `json:"credential"`
	CredentialType TypeCredential `json:"credential_type"`
	ProviderType   TypeProvider   `json:"provider_type"`
	Integration    Integration    `json:"integration"`
	CredentialID   int64          `gorm:"primaryKey" json:"credential_id"`
	UserID         int64          `json:"user_id"`
}
