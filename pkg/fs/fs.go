package fs

import (
	"io"
	"os"
	
	"github.com/oarkflow/bitwise"
	
	"github.com/oarkflow/sftp/pkg/models"
)

// AuthenticationRequest ... An authentication request to the SFTP server.
type AuthenticationRequest struct {
	User          string `json:"username"`
	Pass          string `json:"password"`
	IP            string `json:"ip"`
	SessionID     []byte `json:"session_id"`
	ClientVersion []byte `json:"client_version"`
}

// AuthenticationResponse ... An authentication response from the SFTP server.
type AuthenticationResponse struct {
	Server string      `json:"server"`
	Token  string      `json:"token"`
	User   models.User `json:"user"`
}

// ListerAt ... A list of files.
type ListerAt []os.FileInfo

// ListAt ...
// Returns the number of entries copied and an io.EOF error if we made it to the end of the file list.
// Take a look at the pkg/sftp godoc for more information about how this function should work.
func (l ListerAt) ListAt(f []os.FileInfo, offset int64) (int, error) {
	if offset >= int64(len(l)) {
		return 0, io.EOF
	}
	
	n := copy(f, l[offset:])
	if n < len(f) {
		return n, io.EOF
	}
	return n, nil
}

var factory bitwise.Perman

const (
	// Read ... Permission to read a file.
	Read = "read"
	// ReadContent ... Permission to read the contents of a file.
	ReadContent = "read-content"
	// Create ... Permission to create a file.
	Create = "create"
	// Update ... Permission to update a file.
	Update = "update"
	// Delete ... Permission to delete a file.
	Delete = "delete"
)

func init() {
	factory = bitwise.Factory([]string{Read, ReadContent, Create, Update, Delete})
}

// Can - Determines if a user has permission to perform a specific action on the SFTP server. These
// permissions are defined and returned by the Panel API.
func Can(permissions int64, permission string) bool {
	return factory.Has(permissions, permission)
}

func Serialize(perm []string) int64 {
	return factory.Serialize(perm)
}

func Deserialize(perm int64) []string {
	return factory.Deserialize(perm)
}
