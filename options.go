package sftp

import (
	"github.com/oarkflow/sftp/pkg/fs"
	interfaces2 "github.com/oarkflow/sftp/pkg/providers"
)

func WithUserProvider(provider interfaces2.UserProvider) func(*Server) {
	return func(o *Server) {
		o.userProvider = provider
	}
}

func WithBasePath(path string) func(server *Server) {
	return func(o *Server) {
		o.basePath = path
	}
}

func WithPort(val int) func(server *Server) {
	return func(o *Server) {
		o.port = val
	}
}

func WithAddress(val string) func(server *Server) {
	return func(o *Server) {
		o.address = val
	}
}

func WithSSHPath(val string) func(server *Server) {
	return func(o *Server) {
		o.sshPath = val
	}
}

func WithPrivateKey(val string) func(server *Server) {
	return func(o *Server) {
		o.privateKey = val
	}
}

func WithPublicKey(val string) func(server *Server) {
	return func(o *Server) {
		o.publicKey = val
	}
}

func WithCredentialValidator(val func(server *Server, r fs.AuthenticationRequest) (*fs.AuthenticationResponse, error)) func(server *Server) {
	return func(o *Server) {
		o.credentialValidator = val
	}
}

func WithNotificationCallback(callback NotificationHandler) func(srv *Server) {
	return func(o *Server) {
		o.notificationCallback = callback
	}
}
