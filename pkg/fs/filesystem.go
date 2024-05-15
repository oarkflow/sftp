package fs

import (
	"io"
	
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	
	"github.com/oarkflow/sftp/pkg/log"
)

type FS interface {
	Fileread(request *sftp.Request) (io.ReaderAt, error)
	Filewrite(request *sftp.Request) (io.WriterAt, error)
	Filecmd(request *sftp.Request) error
	Filelist(request *sftp.Request) (sftp.ListerAt, error)
	SetLogger(logger log.Logger)
	Logger() log.Logger
	SetPermissions(p []string)
	Permissions() []string
	SetContext(ctx map[string]string)
	Context() map[string]string
	SetConn(sconn *ssh.ServerConn)
	Conn() *ssh.ServerConn
	SetID(p string)
	Type() string
}
