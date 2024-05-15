package afos

import (
	"errors"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	
	"github.com/oarkflow/sftp/pkg/errs"
	fs2 "github.com/oarkflow/sftp/pkg/fs"
	"github.com/oarkflow/sftp/pkg/log"
)

// Afos ... A file system exposed to a user.
type Afos struct {
	logger        log.Logger
	pathValidator func(fs fs2.FS, p string) (string, error)
	hasDiskSpace  func(fs fs2.FS) bool
	id            string
	basePath      string
	dataPath      string
	permissions   int64
	ctx           map[string]string
	lock          sync.Mutex
	readOnly      bool
	sconn         *ssh.ServerConn
}

func defaultAfos(basePath string) *Afos {
	dataPath := "data"
	return &Afos{
		dataPath: dataPath,
		pathValidator: func(fs fs2.FS, p string) (string, error) {
			join := path.Join(basePath, dataPath, p)
			clean := path.Clean(path.Join(basePath, dataPath))
			if strings.HasPrefix(join, clean) {
				return join, nil
			}
			return "", errors.New("invalid path outside the configured directory was provided")
		},
		hasDiskSpace: func(fs fs2.FS) bool {
			return true // TODO
		},
	}
}

func New(basePath string, opts ...func(*Afos)) fs2.FS {
	svr := defaultAfos(basePath)
	for _, o := range opts {
		o(svr)
	}
	return svr
}

func (f *Afos) SetPermissions(p []string) {
	f.permissions = fs2.Serialize(p)
}

func (f *Afos) Permissions() []string {
	return fs2.Deserialize(f.permissions)
}

func (f *Afos) SetID(p string) {
	f.id = p
}

func (f *Afos) SetContext(ctx map[string]string) {
	f.ctx = ctx
}

func (f *Afos) Context() map[string]string {
	return f.ctx
}

func (f *Afos) buildPath(p string) (string, error) {
	if f.pathValidator == nil {
		return "", nil
	}
	return f.pathValidator(f, p)
}

func (f *Afos) SetLogger(logger log.Logger) {
	f.logger = logger
}

func (f *Afos) Logger() log.Logger {
	return f.logger
}

func (f *Afos) SetConn(sconn *ssh.ServerConn) {
	f.sconn = sconn
}

func (f *Afos) Conn() *ssh.ServerConn {
	return f.sconn
}

// Fileread creates a reader for a file on the system and returns the reader back.
func (f *Afos) Fileread(request *sftp.Request) (io.ReaderAt, error) {
	// Check first if the user can actually open and view a file. This permission is named
	// really poorly, but it is checking if they can read. There is an addition permission,
	// "save-files" which determines if they can write that file.
	if !fs2.Can(f.permissions, fs2.ReadContent) {
		return nil, sftp.ErrSshFxPermissionDenied
	}
	
	p, err := f.buildPath(request.Filepath)
	if err != nil {
		return nil, sftp.ErrSshFxNoSuchFile
	}
	
	f.lock.Lock()
	defer f.lock.Unlock()
	
	if _, err := os.Stat(p); os.IsNotExist(err) {
		return nil, sftp.ErrSshFxNoSuchFile
	}
	
	file, err := os.Open(p)
	if err != nil {
		f.logger.Error("could not open file for reading", "source", p, "err", err)
		return nil, sftp.ErrSshFxFailure
	}
	
	return file, nil
}

// Filewrite handles the write actions for a file on the system.
func (f *Afos) Filewrite(request *sftp.Request) (io.WriterAt, error) {
	if f.readOnly {
		return nil, sftp.ErrSshFxOpUnsupported
	}
	
	p, err := f.buildPath(request.Filepath)
	if err != nil {
		return nil, sftp.ErrSshFxNoSuchFile
	}
	
	// If the user doesn't have enough space left on the server it should respond with an
	// error since we won't be letting them write this file to the disk.
	if f.hasDiskSpace != nil && !f.hasDiskSpace(f) {
		return nil, errs.ErrSSHQuotaExceeded
	}
	
	f.lock.Lock()
	defer f.lock.Unlock()
	
	stat, statErr := os.Stat(p)
	// If the file doesn't exist we need to create it, as well as the directory pathway
	// leading up to where that file will be created.
	if os.IsNotExist(statErr) {
		// This is a different pathway than just editing an existing file. If it doesn't exist already
		// we need to determine if this user has permission to create files.
		if !fs2.Can(f.permissions, fs2.Create) {
			return nil, sftp.ErrSshFxPermissionDenied
		}
		
		// Create all of the directories leading up to the location where this file is being created.
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			f.logger.Error("error making path for file",
				"source", p,
				"path", filepath.Dir(p),
				"err", err,
			)
			return nil, sftp.ErrSshFxFailure
		}
		
		file, err := os.Create(p)
		if err != nil {
			f.logger.Error("error creating file", "source", p, "err", err)
			return nil, sftp.ErrSshFxFailure
		}
		
		return file, nil
	}
	
	// If the stat error isn't about the file not existing, there is some other issue
	// at play and we need to go ahead and bail out of the process.
	if statErr != nil {
		f.logger.Error("error performing file stat", "source", p, "err", err)
		return nil, sftp.ErrSshFxFailure
	}
	
	// If we've made it here it means the file already exists and we don't need to do anything
	// fancy to handle it. Just pass over the request flags so the system knows what the end
	// goal with the file is going to be.
	//
	// But first, check that the user has permission to save modified files.
	if !fs2.Can(f.permissions, fs2.Update) {
		return nil, sftp.ErrSshFxPermissionDenied
	}
	
	// Not sure this would ever happen, but lets not find out.
	if stat.IsDir() {
		f.logger.Warn("attempted to open a directory for writing to", "source", p)
		return nil, sftp.ErrSshFxOpUnsupported
	}
	
	file, err := os.Create(p)
	if err != nil {
		f.logger.Error("error opening existing file",
			"flags", request.Flags,
			"source", p,
			"err", err,
		)
		return nil, sftp.ErrSshFxFailure
	}
	
	return file, nil
}

// Filecmd hander for basic SFTP system calls related to files, but not anything to do with reading
// or writing to those files.
func (f *Afos) Filecmd(request *sftp.Request) error {
	if f.readOnly {
		return sftp.ErrSshFxOpUnsupported
	}
	
	p, err := f.buildPath(request.Filepath)
	if err != nil {
		return sftp.ErrSshFxNoSuchFile
	}
	
	var target string
	// If a target is provided in this request validate that it is going to the correct
	// location for the server. If it is not, return an operation unsupported error. This
	// is maybe not the best error response, but its not wrong either.
	if request.Target != "" {
		target, err = f.buildPath(request.Target)
		if err != nil {
			return sftp.ErrSshFxOpUnsupported
		}
	}
	
	switch request.Method {
	case "Setstat":
		if !fs2.Can(f.permissions, fs2.Update) {
			return sftp.ErrSshFxPermissionDenied
		}
		
		var mode os.FileMode = 0644
		// If the client passed a valid file permission use that, otherwise use the
		// default of 0644 set above.
		if request.Attributes().FileMode().Perm() != 0000 {
			mode = request.Attributes().FileMode().Perm()
		}
		
		// Force directories to be 0755
		if request.Attributes().FileMode().IsDir() {
			mode = 0755
		}
		
		if err := os.Chmod(p, mode); err != nil {
			f.logger.Error("failed to perform setstat", "err", err)
			return sftp.ErrSshFxFailure
		}
		return nil
	case "Rename":
		if !fs2.Can(f.permissions, fs2.Update) {
			return sftp.ErrSshFxPermissionDenied
		}
		
		if err := os.Rename(p, target); err != nil {
			f.logger.Error("failed to rename file",
				"source", p,
				"target", target,
				"err", err,
			)
			return sftp.ErrSshFxFailure
		}
		
		break
	case "Rmdir":
		if !fs2.Can(f.permissions, fs2.Delete) {
			return sftp.ErrSshFxPermissionDenied
		}
		
		if err := os.RemoveAll(p); err != nil {
			f.logger.Error("failed to remove directory", "source", p, "err", err)
			return sftp.ErrSshFxFailure
		}
		
		return sftp.ErrSshFxOk
	case "Mkdir":
		if !fs2.Can(f.permissions, fs2.Create) {
			return sftp.ErrSshFxPermissionDenied
		}
		
		if err := os.MkdirAll(p, 0755); err != nil {
			f.logger.Error("failed to create directory", "source", p, "err", err)
			return sftp.ErrSshFxFailure
		}
		
		break
	case "Symlink":
		if !fs2.Can(f.permissions, fs2.Create) {
			return sftp.ErrSshFxPermissionDenied
		}
		
		if err := os.Symlink(p, target); err != nil {
			f.logger.Error("failed to create symlink",
				"source", p, "err", err,
				"target", target,
			)
			return sftp.ErrSshFxFailure
		}
		
		break
	case "Remove":
		if !fs2.Can(f.permissions, fs2.Delete) {
			return sftp.ErrSshFxPermissionDenied
		}
		
		if err := os.Remove(p); err != nil {
			if !os.IsNotExist(err) {
				f.logger.Error("failed to remove a file", "source", p, "err", err)
			}
			return sftp.ErrSshFxFailure
		}
		
		return sftp.ErrSshFxOk
	default:
		return sftp.ErrSshFxOpUnsupported
	}
	
	return sftp.ErrSshFxOk
}

// Filelist is the handler for SFTP filesystem list calls. This will handle calls to list the contents of
// a directory as well as perform file/folder stat calls.
func (f *Afos) Filelist(request *sftp.Request) (sftp.ListerAt, error) {
	p, err := f.buildPath(request.Filepath)
	if err != nil {
		return nil, sftp.ErrSshFxNoSuchFile
	}
	
	switch request.Method {
	case "List":
		if !fs2.Can(f.permissions, fs2.Read) {
			return nil, sftp.ErrSshFxPermissionDenied
		}
		
		files, err := ioutil.ReadDir(p)
		if err != nil {
			f.logger.Error("error listing directory", "err", err)
			return nil, sftp.ErrSshFxFailure
		}
		return fs2.ListerAt(files), nil
	case "Stat":
		if !fs2.Can(f.permissions, fs2.Read) {
			return nil, sftp.ErrSshFxPermissionDenied
		}
		
		s, err := os.Stat(p)
		if os.IsNotExist(err) {
			return nil, sftp.ErrSshFxNoSuchFile
		} else if err != nil {
			f.logger.Error("error running STAT on file", "err", err)
			return nil, sftp.ErrSshFxFailure
		}
		
		return fs2.ListerAt([]os.FileInfo{s}), nil
	default:
		// Before adding readlink support we need to evaluate any potential security risks
		// as a result of navigating around to a location that is outside the home directory
		// for the logged in user. I don't forsee it being much of a problem, but I do want to
		// check it out before slapping some code here. Until then, we'll just return an
		// unsupported response code.
		return nil, sftp.ErrSshFxOpUnsupported
	}
}

func (f *Afos) Type() string {
	return "os"
}
