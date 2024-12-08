package server

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os/user"
	"path"
	"time"

	"golang.org/x/crypto/ssh"
)

type SSHInfo struct {
	Host     string `json:"host"`
	User     string `json:"user"`
	Port     int    `json:"port"`
	Password string `json:"password"`
}

func (info *SSHInfo) Address() string {
	return fmt.Sprintf("%s:%d", info.Host, info.Port)
}

func connectSSH(info *SSHInfo) (*ssh.Client, error) {
	config := &ssh.ClientConfig{
		User: info.User,
		Auth: []ssh.AuthMethod{
			ssh.Password(info.Password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}
	return ssh.Dial("tcp", info.Address(), config)
}

func defaultVerifyDir() (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", err
	}
	dir := path.Join(u.HomeDir, ".tune", "verify")
	return dir, nil
}

func loadSavedHosts() ([]SSHInfo, error) {
	dir, err := defaultVerifyDir()
	if err != nil {
		return nil, err
	}
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var hosts []SSHInfo
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		if len(f.Name()) < 5 {
			continue
		}
		// ファイル名: ssh-<host>.json
		// ファイル読み込み
		if f.Name()[0:4] == "ssh-" && path.Ext(f.Name()) == ".json" {
			data, err := ioutil.ReadFile(path.Join(dir, f.Name()))
			if err != nil {
				continue
			}
			info, err := parseSSHInfoJSON(data)
			if err != nil {
				continue
			}
			hosts = append(hosts, info)
		}
	}
	return hosts, nil
}

func parseSSHInfoJSON(data []byte) (SSHInfo, error) {
	info, err := parseJSONToSSHInfo(data)
	if err != nil {
		return SSHInfo{}, err
	}
	if info.Host == "" || info.User == "" || info.Port == 0 || info.Password == "" {
		return SSHInfo{}, errors.New("不正なSSH情報")
	}
	return info, nil
}
