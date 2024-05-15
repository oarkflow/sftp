package s3

import (
	"context"
	"io"
	"os"
	"strings"
	
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	
	fs2 "github.com/oarkflow/sftp/pkg/fs"
	"github.com/oarkflow/sftp/pkg/log"
)

func (f *Fs) SetContext(ctx map[string]string) {
	f.ctx = ctx
}

func (f *Fs) Context() map[string]string {
	return f.ctx
}

func (f *Fs) Logger() log.Logger {
	return f.logger
}

func (f *Fs) SetConn(sconn *ssh.ServerConn) {
	f.sconn = sconn
}

func (f *Fs) Conn() *ssh.ServerConn {
	return f.sconn
}

func (f *Fs) Fileread(request *sftp.Request) (io.ReaderAt, error) {
	if !fs2.Can(f.permissions, fs2.ReadContent) {
		return nil, sftp.ErrSshFxPermissionDenied
	}
	switch request.Method {
	case "Get":
		key := strings.TrimPrefix(request.Filepath, "/")
		object, err := f.client.GetObject(context.Background(), &s3.GetObjectInput{
			Bucket: aws.String(f.bucket),
			Key:    aws.String(key),
		})
		if err != nil {
			return nil, err
		}
		return reader{object: object, client: f.client, key: key, bucket: f.bucket}, nil
	default:
		return nil, sftp.ErrSSHFxOpUnsupported
	}
}

func (f *Fs) Filewrite(request *sftp.Request) (io.WriterAt, error) {
	if f.readOnly {
		return nil, sftp.ErrSshFxOpUnsupported
	}
	switch request.Method {
	case "Put":
		return newWriter(context.Background(), f.client, f.bucket, strings.TrimPrefix(request.Filepath, "/"))
	default:
		return nil, sftp.ErrSSHFxOpUnsupported
	}
}

func (f *Fs) Filecmd(request *sftp.Request) error {
	if f.readOnly {
		return sftp.ErrSshFxOpUnsupported
	}
	p := request.Filepath
	target := request.Target
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
		
		if err := f.Chmod(p, mode); err != nil {
			f.logger.Error("failed to perform setstat", "err", err)
			return sftp.ErrSshFxFailure
		}
		return nil
	case "Rename":
		if !fs2.Can(f.permissions, fs2.Update) {
			return sftp.ErrSshFxPermissionDenied
		}
		
		if err := f.Rename(p, target); err != nil {
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
		
		if err := f.RemoveAll(p); err != nil {
			f.logger.Error("failed to remove directory", "source", p, "err", err)
			return sftp.ErrSshFxFailure
		}
		
		return sftp.ErrSshFxOk
	case "Mkdir":
		if !fs2.Can(f.permissions, fs2.Create) {
			return sftp.ErrSshFxPermissionDenied
		}
		
		if err := f.MkdirAll(p, 0755); err != nil {
			f.logger.Error("failed to create directory", "source", p, "err", err)
			return sftp.ErrSshFxFailure
		}
		
		break
	case "Remove":
		if !fs2.Can(f.permissions, fs2.Delete) {
			return sftp.ErrSshFxPermissionDenied
		}
		
		if err := f.Remove(p); err != nil {
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

func (f *Fs) Filelist(request *sftp.Request) (sftp.ListerAt, error) {
	p := request.Filepath
	switch request.Method {
	case "List":
		if !fs2.Can(f.permissions, fs2.Read) {
			return nil, sftp.ErrSshFxPermissionDenied
		}
		file := NewFile(f, p)
		files, err := file.ReaddirAll()
		if err != nil {
			f.logger.Error("error listing directory", "err", err)
			return nil, sftp.ErrSshFxFailure
		}
		
		return fs2.ListerAt(files), nil
	case "Stat":
		if !fs2.Can(f.permissions, fs2.Read) {
			return nil, sftp.ErrSshFxPermissionDenied
		}
		
		s, err := f.Stat(p)
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

func (f *Fs) SetLogger(logger log.Logger) {
	f.logger = logger
}

func (f *Fs) SetPermissions(p []string) {
	f.permissions = fs2.Serialize(p)
}

func (f *Fs) Permissions() []string {
	return fs2.Deserialize(f.permissions)
}

func (f *Fs) SetID(p string) {
	f.id = p
}

func (f *Fs) Type() string {
	return "s3"
}

type Option struct {
	Endpoint  string `json:"endpoint"`
	Region    string `json:"region"`
	Bucket    string `json:"bucket"`
	AccessKey string `json:"access_key"`
	Secret    string `json:"secret"`
}

func New(opt Option) (fs2.FS, error) {
	creds := aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(opt.AccessKey, opt.Secret, ""))
	conf := aws.Config{
		Credentials: creds,
		Region:      opt.Region,
		EndpointResolverWithOptions: aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL:               opt.Endpoint,
				SigningRegion:     opt.Region,
				HostnameImmutable: true,
			}, nil
		}),
	}
	
	s3Fs := NewFsFromConfig(opt.Bucket, conf)
	return s3Fs, nil
}
