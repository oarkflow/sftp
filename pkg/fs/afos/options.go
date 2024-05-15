package afos

import (
	fs2 "github.com/oarkflow/sftp/pkg/fs"
)

func WithDataPath(val string) func(server *Afos) {
	return func(o *Afos) {
		o.dataPath = val
	}
}

func WithPermissions(val []string) func(server *Afos) {
	return func(o *Afos) {
		o.permissions = fs2.Serialize(val)
	}
}

func WithReadOnly(val bool) func(server *Afos) {
	return func(o *Afos) {
		o.readOnly = val
	}
}

func WithDiskSpaceValidator(val func(fs fs2.FS) bool) func(server *Afos) {
	return func(o *Afos) {
		o.hasDiskSpace = val
	}
}

func WithPathValidator(val func(fs fs2.FS, p string) (string, error)) func(server *Afos) {
	return func(o *Afos) {
		o.pathValidator = val
	}
}
