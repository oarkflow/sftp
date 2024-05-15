package providers

import (
	"crypto/rand"
	"math/big"
	"sync"
	
	"github.com/oarkflow/hash"
	
	"github.com/oarkflow/sftp/pkg/errs"
	"github.com/oarkflow/sftp/pkg/fs"
	"github.com/oarkflow/sftp/pkg/models"
)

type JsonFileProvider struct {
	users             map[string]models.User
	hashAlgo          string
	alternateHashAlgo string
	mu                sync.RWMutex
}

func (p *JsonFileProvider) Login(username, pass string) (*fs.AuthenticationResponse, error) {
	user, exists := p.users[username]
	matched, err := hash.Match(pass, user.Password, p.hashAlgo)
	if !exists || err != nil || !matched {
		return nil, errs.InvalidCredentialsError{}
	}
	n, _ := rand.Int(rand.Reader, big.NewInt(9223372036854775807))
	return &fs.AuthenticationResponse{
		Server: "none",
		Token:  n.String(),
		User:   user,
	}, nil
}

func (p *JsonFileProvider) Register(user models.User) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.users[user.Username] = user
}

func NewJsonFileProvider(hashAlgo, alternateHashAlgo string, users ...map[string]models.User) *JsonFileProvider {
	user := make(map[string]models.User)
	if len(users) > 0 && users[0] != nil {
		user = users[0]
	}
	if hashAlgo == "" {
		hashAlgo = "sha256"
	}
	return &JsonFileProvider{users: user, hashAlgo: hashAlgo, alternateHashAlgo: alternateHashAlgo}
}
