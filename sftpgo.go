package sftpgo

import (
	"encoding/base64"
	"fmt"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Host, Username, Password string
	Port                     int
	SSHTrustedKey            string
}

type SftpClient interface {
	OpenFile(remoteFile string) (*sftp.File, error)
	PutFile(localFile, remoteFile string) (err error)
	PutString(text, remoteFile string) (err error)
	GetRecords(remoteFile string, onReader func(file *sftp.File) ([][]string, error)) ([][]string, error)
	Files(remoteDir string) []os.FileInfo
	WalkFiles(remoteDir string) ([]string, error)
	MoveFile(sourcePath string, destPath string) error
	RemoveFile(dirPath string) error
	Close() error
	ConnectionLostHandler(err error)
}

type sftpClient struct {
	Client          *sftp.Client
	reconnect       chan bool
	ReconnectStatus chan bool
}

// NewClient Create a new SFTP connection by given parameters
func NewClient(conn SftpConn) (SftpClient, error) {
	client, err := conn.Connect()

	reconnect := make(chan bool)
	status := make(chan bool)
	ftpConn := &sftpClient{Client: client, reconnect: reconnect, ReconnectStatus: status}
	go func() {
		// Receive connection when connection lose
		for {
			select {
			case res := <-reconnect:
				if res {
					newConn, e := conn.Connect()
					if e == nil {
						ftpConn.Client = newConn
						ftpConn.ReconnectStatus <- true
						slog.Info("Reconnected")
					} else {
						ftpConn.ReconnectStatus <- false
						slog.Error("Reconnect failure", e)
					}
				}
			}
		}
	}()

	return ftpConn, err
}

func (sc *sftpClient) ConnectionLostHandler(err error) {
	if strings.Contains(err.Error(), "connection lost") {
		slog.Info("Reconnecting...")
		sc.reconnect <- true
	}
}

func (sc *sftpClient) MoveFile(sourcePath string, destPath string) error {
	err := sc.Client.Rename(sourcePath, destPath)
	sc.ConnectionLostHandler(err)
	return err
}

// WalkFiles
// filePaths, err := sc.WalkFiles(remoteDir)
func (sc *sftpClient) WalkFiles(remoteDir string) ([]string, error) {
	files := []string{}

	walker := sc.Client.Walk(remoteDir)
	for walker.Step() {
		if err := walker.Err(); err != nil {
			sc.ConnectionLostHandler(err)
			return files, err
		}

		if !(walker.Stat().IsDir()) {
			files = append(files, walker.Path())
		}
	}

	return files, nil
}

func (sc *sftpClient) OpenFile(remoteFile string) (*sftp.File, error) {
	// Open the file
	file, err := sc.Client.Open(remoteFile)
	if err != nil {
		slog.Error(fmt.Sprintf("Error opening file %s: %v", remoteFile, err))
		sc.ConnectionLostHandler(err)
		return nil, err
	}

	return file, nil
}

func (sc *sftpClient) GetRecords(remoteFile string, onReader func(file *sftp.File) ([][]string, error)) ([][]string, error) {
	// Open the file
	file, err := sc.OpenFile(remoteFile)
	if err == nil {
		defer func(file *sftp.File) { _ = file.Close() }(file)
	} else {
		sc.ConnectionLostHandler(err)
	}

	// Read the file contents
	contents, err := onReader(file)
	if err != nil {
		slog.Error(fmt.Sprintf("Error reading file %s: %v", remoteFile, err))
		return [][]string{}, err
	}
	return contents, nil
}

func (sc *sftpClient) Files(remoteDir string) []os.FileInfo {
	entries, err := sc.Client.ReadDir(remoteDir)
	if err != nil {
		slog.Error("Failed to read directory:", err)
		sc.ConnectionLostHandler(err)
		return []os.FileInfo{}
	}
	return entries
}

func (sc *sftpClient) RemoveFile(dirPath string) error {
	err := sc.Client.Remove(dirPath)
	sc.ConnectionLostHandler(err)
	return err
}

// PutFile Upload file to sftp server
func (sc *sftpClient) PutFile(localFile, remoteFile string) (err error) {
	srcFile, err := os.Open(localFile)
	if err != nil {
		slog.Error("Open file", err)
		sc.ConnectionLostHandler(err)
		return err
	}
	defer func(srcFile *os.File) { _ = srcFile.Close() }(srcFile)

	// Make remote directories recursion
	parent := filepath.Dir(remoteFile)
	path := string(filepath.Separator)
	dirs := strings.Split(parent, path)
	for _, dir := range dirs {
		path = filepath.Join(path, dir)
		_ = sc.Client.Mkdir(path)
	}

	dstFile, err := sc.Client.Create(remoteFile)
	if err != nil {
		slog.Error("Create remote file", err)
		sc.ConnectionLostHandler(err)
		return err
	}
	defer func(dstFile *sftp.File) { _ = dstFile.Close() }(dstFile)

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// PutString Upload message to sftp server
func (sc *sftpClient) PutString(text, remoteFile string) (err error) {
	// Make remote directories recursion
	parent := filepath.Dir(remoteFile)
	path := string(filepath.Separator)
	dirs := strings.Split(parent, path)
	for _, dir := range dirs {
		path = filepath.Join(path, dir)
		_ = sc.Client.Mkdir(path)
	}

	// Open a file on the remote SFTP server for writing
	dstFile, err := sc.Client.Create(remoteFile)
	if err != nil {
		slog.Error("Open a file on the remote:", err)
		sc.ConnectionLostHandler(err)
	}
	defer func(dstFile *sftp.File) { _ = dstFile.Close() }(dstFile)

	// Write the string text to the remote file
	_, err = io.Copy(dstFile, strings.NewReader(text))
	if err != nil {
		slog.Error("Copy a text to remote:", err)
	}
	return nil
}

func (sc *sftpClient) Close() error {
	return sc.Client.Close()
}

// SSH Key-strings
func trustedHostKeyCallback(trustedKey string) ssh.HostKeyCallback {

	if trustedKey == "" {
		return func(_ string, _ net.Addr, k ssh.PublicKey) error {
			slog.Error(fmt.Sprintf("WARNING: SSH-key verification is *NOT* in effect: to fix, add this trustedKey: %q", keyString(k)))
			return nil
		}
	}

	return func(_ string, _ net.Addr, k ssh.PublicKey) error {
		ks := keyString(k)
		if trustedKey != ks {
			return fmt.Errorf("SSH-key verification: expected %q but got %q", trustedKey, ks)
		}

		return nil
	}
}

func keyString(k ssh.PublicKey) string {
	return k.Type() + " " + base64.StdEncoding.EncodeToString(k.Marshal())
}
