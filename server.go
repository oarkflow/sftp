package sftp

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"slices"
	"time"
	
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	
	"github.com/oarkflow/sftp/pkg/fs"
	"github.com/oarkflow/sftp/pkg/log"
	"github.com/oarkflow/sftp/pkg/log/oarklog"
	"github.com/oarkflow/sftp/pkg/models"
	providers2 "github.com/oarkflow/sftp/pkg/providers"
	"github.com/oarkflow/sftp/pkg/utils"
)

type NotificationHandler func(notification Notification) error
type Server struct {
	userProvider         providers2.UserProvider
	logger               log.Logger
	credentialValidator  func(server *Server, r fs.AuthenticationRequest) (*fs.AuthenticationResponse, error)
	notificationCallback NotificationHandler
	basePath             string
	sshPath              string
	privateKey           string
	publicKey            string
	address              string
	port                 int
	notify               bool
}

func defaultServer() *Server {
	basePath := utils.AbsPath("")
	userProvider := providers2.NewJsonFileProvider("sha256", "")
	return &Server{
		basePath:     basePath,
		sshPath:      ".ssh",
		port:         2022,
		privateKey:   "id_rsa",
		publicKey:    "id_rsa.pub",
		address:      "0.0.0.0",
		logger:       oarklog.Default(),
		notify:       true,
		userProvider: userProvider,
		credentialValidator: func(server *Server, r fs.AuthenticationRequest) (*fs.AuthenticationResponse, error) {
			return server.userProvider.Login(r.User, r.Pass)
		},
	}
}

func New(opts ...func(*Server)) *Server {
	svr := defaultServer()
	return newServer(svr, opts...)
}

func NewWithNotify(opts ...func(*Server)) *Server {
	svr := defaultServer()
	svr.notify = true
	return newServer(svr, opts...)
}

func newServer(svr *Server, opts ...func(*Server)) *Server {
	for _, o := range opts {
		o(svr)
	}
	return svr
}

func (c *Server) AddUser(user models.User) {
	c.userProvider.Register(user)
}

func (c *Server) Validate(conn ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
	now := time.Now().UTC()
	nowString := now.Format(time.RFC3339)
	user := conn.User()
	clientVersion := string(conn.ClientVersion())
	remoteAddr := conn.RemoteAddr().String()
	sessionID := conn.SessionID()
	resp, err := c.credentialValidator(c, fs.AuthenticationRequest{
		User:          user,
		Pass:          string(pass),
		IP:            remoteAddr,
		SessionID:     sessionID,
		ClientVersion: conn.ClientVersion(),
	})
	
	if err != nil {
		return nil, err
	}
	fst, err := resp.User.GetFilesystem()
	if err != nil {
		return nil, err
	}
	useDefaultFS := "false"
	var filesystem string
	if fst != nil {
		fsBytes, err := json.Marshal(fst)
		if err != nil {
			return nil, err
		}
		filesystem = string(fsBytes)
	} else {
		useDefaultFS = "true"
	}
	c.logger.Info("User Authenticated",
		"user", user,
		"login_at", nowString,
		"event", "Login",
		"remote_addr", remoteAddr,
		"client_version", clientVersion,
		"fs_type", fst.Fs,
	)
	if c.notify && c.notificationCallback != nil {
		c.notificationCallback(Notification{
			User:          user,
			ClientVersion: clientVersion,
			RemoteAddr:    remoteAddr,
			Time:          now,
			Event:         "Login",
			FsType:        fst.Fs,
		})
	}
	sshPerm := &ssh.Permissions{
		Extensions: map[string]string{
			"uuid":           resp.Server,
			"user":           user,
			"remote_addr":    remoteAddr,
			"filesystem":     filesystem,
			"default_fs":     useDefaultFS,
			"client_version": clientVersion,
			"login_at":       nowString,
		},
	}
	return sshPerm, nil
}

// Initialize the SFTP server and add a persistent listener to handle inbound SFTP connections.
func (c *Server) Initialize() error {
	config, err := c.setupSSH()
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", c.address, c.port))
	if err != nil {
		return err
	}
	
	c.logger.Info("Listening connections", "host", c.address, "port", c.port)
	
	for {
		conn, _ := listener.Accept()
		if conn != nil {
			go c.AcceptInboundConnection(conn, config)
		}
	}
}

// AcceptInboundConnection ... Handles an inbound connection to the instance and determines if
// we should serve the request or not.
func (c *Server) AcceptInboundConnection(conn net.Conn, config *ssh.ServerConfig) {
	defer conn.Close()
	sconn, chans, reqs, err := ssh.NewServerConn(conn, config)
	if err != nil {
		return
	}
	defer sconn.Close()
	go ssh.DiscardRequests(reqs)
	for newChannel := range chans {
		if newChannel.ChannelType() != "session" {
			newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
			continue
		}
		channel, requests, err := newChannel.Accept()
		if err != nil {
			continue
		}
		go func(in <-chan *ssh.Request) {
			for req := range in {
				ok := false
				switch req.Type {
				case "subsystem":
					if string(req.Payload[4:]) == "sftp" {
						ok = true
					}
				}
				req.Reply(ok, nil)
			}
		}(requests)
		if sconn.Permissions.Extensions["uuid"] == "" {
			continue
		}
		handlers, err := c.createHandler(sconn)
		if err != nil {
			newChannel.Reject(ssh.ConnectionFailed, err.Error())
			channel.Close()
			return
		}
		server := sftp.NewRequestServer(channel, handlers)
		if err := server.Serve(); err == io.EOF {
			server.Close()
		}
	}
}

// Creates a new SFTP handler for a given server. The directory argument should
// be the base directory for a server. All actions done on the server will be
// relative to that directory, and the user will not be able to escape out of it.
func (c *Server) createHandler(sconn *ssh.ServerConn) (sftp.Handlers, error) {
	fst, err := c.getUserFilesystem(sconn, c.basePath)
	if err != nil {
		return sftp.Handlers{}, err
	}
	if c.notify {
		fst = NewFS(fst, c.notificationCallback)
	}
	ext := sconn.Permissions.Extensions
	ctx := make(map[string]string)
	for key, val := range ext {
		if !slices.Contains([]string{"filesystem", "default_fs", "server_version", "login_at", "uuid"}, key) {
			ctx[key] = val
		}
	}
	fst.SetConn(sconn)
	fst.SetContext(ctx)
	fst.SetID(ext["uuid"])
	return sftp.Handlers{FileGet: fst, FilePut: fst, FileCmd: fst, FileList: fst}, nil
}

func (c *Server) getSSHPath(file string) string {
	return path.Join(c.basePath, c.sshPath, file)
}

func (c *Server) setupSSH() (*ssh.ServerConfig, error) {
	config := &ssh.ServerConfig{
		NoClientAuth:     false,
		MaxAuthTries:     6,
		PasswordCallback: c.Validate,
	}
	if _, err := os.Stat(c.getSSHPath(c.privateKey)); os.IsNotExist(err) {
		if err := c.generatePrivateKey(); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	
	privateBytes, err := os.ReadFile(c.getSSHPath(c.privateKey))
	if err != nil {
		return nil, err
	}
	
	private, err := ssh.ParsePrivateKey(privateBytes)
	if err != nil {
		return nil, err
	}
	
	// Add our private key to the server configuration.
	config.AddHostKey(private)
	/*err = c.generatePublicKey(privateBytes)
	if err != nil {
		return nil, err
	}*/
	return config, nil
}

// Generates a private key that will be used by the SFTP server.
func (c *Server) generatePrivateKey() error {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}
	
	if err := os.MkdirAll(path.Join(c.basePath, c.sshPath), 0755); err != nil {
		return err
	}
	
	o, err := os.OpenFile(c.getSSHPath(c.privateKey), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer o.Close()
	
	pkey := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}
	return pem.Encode(o, pkey)
}

// GetPublicKey extracts the public key from an ssh.Signer (typically a private key)
func (c *Server) generatePublicKey(privKey []byte) error {
	_, err := os.Stat(c.getSSHPath(c.publicKey))
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	}
	
	block, _ := pem.Decode(privKey)
	if block == nil || block.Type != "RSA PRIVATE KEY" {
		return errors.New("failed to decode PEM block containing public key")
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return err
	}
	
	// publicKeyDer := x509.MarshalPKCS1PublicKey(&key.PublicKey)
	publicKeyDer, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return err
	}
	o, err := os.OpenFile(c.getSSHPath(c.publicKey), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer o.Close()
	pubKeyBlock := &pem.Block{
		Type:    "PUBLIC KEY",
		Headers: nil,
		Bytes:   publicKeyDer,
	}
	return pem.Encode(o, pubKeyBlock)
}
