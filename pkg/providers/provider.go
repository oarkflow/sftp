package providers

import (
	"github.com/oarkflow/sftp/pkg/fs"
	"github.com/oarkflow/sftp/pkg/models"
)

var (
	DefaultPermissions = []string{"file.read", "file.read-content", "file.create", "file.update", "file.delete"}
)

type UserProvider interface {
	Login(user, pass string) (*fs.AuthenticationResponse, error)
	Register(user models.User)
}
