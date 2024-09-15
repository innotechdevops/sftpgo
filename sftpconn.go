package sftpgo

import (
	"fmt"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"log/slog"
	"time"
)

type SftpConn interface {
	Connect() (client *sftp.Client, err error)
}

type sftpConn struct {
	Config *Config
}

func (s *sftpConn) Connect() (client *sftp.Client, err error) {
	config := &ssh.ClientConfig{
		User:    s.Config.Username,
		Auth:    []ssh.AuthMethod{ssh.Password(s.Config.Password)},
		Timeout: 30 * time.Second,
	}
	if s.Config.SSHTrustedKey != "" {
		config.HostKeyCallback = trustedHostKeyCallback(s.Config.SSHTrustedKey)
	} else {
		config.HostKeyCallback = ssh.InsecureIgnoreHostKey()
	}

	// Connect to ssh
	addr := fmt.Sprintf("%s:%d", s.Config.Host, s.Config.Port)
	conn, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		slog.Error("Connect fail:", err)
		return nil, err
	}

	// Create sftp client
	client, err = sftp.NewClient(conn)
	if err != nil {
		slog.Error("New client fail:", err)
		return nil, err
	}

	return client, nil
}

func NewSftpConn(c *Config) SftpConn {
	return &sftpConn{
		Config: c,
	}
}
