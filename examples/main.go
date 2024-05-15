package main

import (
	"encoding/json"
	"os"
	
	"github.com/oarkflow/sftp/pkg/models"
)

type config struct {
	Address  string `json:"address"`
	Filepath string `json:"files"`
	Port     int    `json:"port"`
	ReadOnly bool   `json:"readOnly"`
}

func main() {
	var users map[string]models.User
	var conf config
	configFile, err := os.ReadFile("config.json")
	if err != nil {
		panic(err)
	}
	err = json.Unmarshal(configFile, &conf)
	if err != nil {
		panic(err)
	}
	usersFile, err := os.ReadFile("users.json")
	if err != nil {
		panic(err)
	}
	err = json.Unmarshal(usersFile, &users)
	if err != nil {
		panic(err)
	}
	server := ftpserver.NewWithNotify()
	for _, user := range users {
		server.AddUser(user)
	}
	panic(server.Initialize())
}
