package sftp

import (
	"encoding/json"
	"errors"
	"io"
	"time"
	
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	
	"github.com/oarkflow/sftp/pkg/fs"
	"github.com/oarkflow/sftp/pkg/fs/afos"
	"github.com/oarkflow/sftp/pkg/fs/s3"
	"github.com/oarkflow/sftp/pkg/log"
	"github.com/oarkflow/sftp/pkg/models"
	"github.com/oarkflow/sftp/pkg/providers"
)

type FS struct {
	fs       fs.FS
	callback NotificationHandler
}

func NewFS(fs fs.FS, callback NotificationHandler) fs.FS {
	return &FS{fs: fs, callback: callback}
}

func (f *FS) SetContext(ctx map[string]string) {
	f.fs.SetContext(ctx)
}

func (f *FS) Context() map[string]string {
	return f.fs.Context()
}

type Notification struct {
	User          string    `json:"user"`
	FsType        string    `json:"fs_type"`
	ClientVersion string    `json:"client_version"`
	RemoteAddr    string    `json:"remote_addr"`
	Time          time.Time `json:"time"`
	Event         string    `json:"event"`
	Subject       string    `json:"subject"`
	Target        string    `json:"target"`
	Error         error     `json:"error"`
}

func (f *FS) Notify(request *sftp.Request, err error) {
	method := request.Method
	if method == "List" {
		return
	}
	notification := Notification{Time: time.Now().UTC(), FsType: f.Type()}
	keyvals := []any{"fs_type", f.Type()}
	for key, val := range f.fs.Context() {
		if key == "user" {
			notification.User = val
		}
		if key == "client_version" {
			notification.ClientVersion = val
		}
		if key == "remote_addr" {
			notification.RemoteAddr = val
		}
		keyvals = append(keyvals, key, val)
	}
	notification.Event = method
	notification.Subject = request.Filepath
	keyvals = append(keyvals, "event", method, "subject", request.Filepath)
	notification.Target = request.Target
	if request.Target != "" {
		keyvals = append(keyvals, "target", request.Target)
	}
	if err != nil && !errors.Is(err, sftp.ErrSshFxOk) {
		keyvals = append(keyvals, "error", err)
		notification.Error = err
		f.fs.Logger().Error("SFTP Event Triggered", keyvals...)
	} else {
		f.fs.Logger().Info("SFTP Event Triggered", keyvals...)
	}
	if f.callback != nil {
		f.callback(notification)
	}
}

func (f *FS) Fileread(request *sftp.Request) (io.ReaderAt, error) {
	var err error
	defer func() {
		f.Notify(request, err)
	}()
	rs, e := f.fs.Fileread(request)
	err = e
	return rs, e
}

func (f *FS) Filewrite(request *sftp.Request) (io.WriterAt, error) {
	var err error
	defer func() {
		f.Notify(request, err)
	}()
	rs, e := f.fs.Filewrite(request)
	err = e
	return rs, e
}

func (f *FS) Filecmd(request *sftp.Request) error {
	var err error
	defer func() {
		f.Notify(request, err)
	}()
	e := f.fs.Filecmd(request)
	err = e
	return e
}

func (f *FS) Filelist(request *sftp.Request) (sftp.ListerAt, error) {
	var err error
	defer func() {
		f.Notify(request, err)
	}()
	rs, e := f.fs.Filelist(request)
	err = e
	return rs, e
}

func (f *FS) SetLogger(logger log.Logger) {
	f.fs.SetLogger(logger)
}

func (f *FS) Logger() log.Logger {
	return f.fs.Logger()
}

func (f *FS) SetPermissions(p []string) {
	f.fs.SetPermissions(p)
}

func (f *FS) Permissions() []string {
	return f.fs.Permissions()
}

func (f *FS) SetConn(sconn *ssh.ServerConn) {
	f.fs.SetConn(sconn)
}

func (f *FS) Conn() *ssh.ServerConn {
	return f.fs.Conn()
}

func (f *FS) SetID(p string) {
	f.fs.SetID(p)
}

func (f *FS) Type() string {
	return f.fs.Type()
}

func (c *Server) getUserFilesystem(sconn *ssh.ServerConn, path string) (fs.FS, error) {
	var userFS models.Filesystem
	if useDefaultFS, exists := sconn.Permissions.Extensions["default_fs"]; exists && useDefaultFS == "true" {
		fst := afos.New(path)
		fst.SetLogger(c.logger)
		fst.SetPermissions(providers.DefaultPermissions)
		return fst, nil
	}
	
	err := json.Unmarshal([]byte(sconn.Permissions.Extensions["filesystem"]), &userFS)
	if err != nil {
		fst := afos.New(path)
		fst.SetLogger(c.logger)
		fst.SetPermissions(providers.DefaultPermissions)
		return fst, nil
	}
	permissions := userFS.Permissions
	if len(userFS.Permissions) == 0 {
		permissions = providers.DefaultPermissions
	}
	switch userFS.Fs {
	case "s3":
		var endpoint, region, bucket, accessKey, secret string
		if val, exists := userFS.Params["endpoint"]; exists {
			endpoint = val.(string)
		}
		if val, exists := userFS.Params["region"]; exists {
			region = val.(string)
		} else {
			region = "us-east-1"
		}
		if val, exists := userFS.Params["bucket"]; exists {
			bucket = val.(string)
		}
		if val, exists := userFS.Params["access_key"]; exists {
			accessKey = val.(string)
		}
		if val, exists := userFS.Params["secret"]; exists {
			secret = val.(string)
		}
		opt := s3.Option{
			Endpoint:  endpoint,
			Region:    region,
			Bucket:    bucket,
			AccessKey: accessKey,
			Secret:    secret,
		}
		fst, err := s3.New(opt)
		if err != nil {
			return nil, err
		}
		fst.SetLogger(c.logger)
		fst.SetPermissions(permissions)
		return fst, nil
	case "os":
		basePath := ""
		if val, exists := userFS.Params["base_path"]; exists {
			basePath = val.(string)
		}
		fst := afos.New(basePath)
		fst.SetLogger(c.logger)
		fst.SetPermissions(permissions)
		return fst, nil
	}
	fst := afos.New(path)
	fst.SetLogger(c.logger)
	fst.SetPermissions(providers.DefaultPermissions)
	return fst, nil
}
