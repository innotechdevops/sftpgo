package main

import (
	"fmt"
	"github.com/innotechdevops/sftpgo"
	"log"
	"log/slog"
	"net/http"
	"os"
)

func New() sftpgo.SftpClient {
	config := sftpgo.Config{
		Host:     os.Getenv("HOST"),
		Username: os.Getenv("USER"),
		Password: os.Getenv("PASS"),
		Port:     2022,
	}
	ftpConn := sftpgo.NewSftpConn(&config)
	csftp, err := sftpgo.NewClient(ftpConn)
	if err != nil {
		log.Panic("New SFTP client", err.Error())
	}
	return csftp
}

func main() {
	client := New()
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		dirs, err := client.WalkFiles("pts/chiller")
		if err != nil {
			slog.Error(err.Error())
		}
		_, _ = fmt.Fprintf(w, "%s", dirs)
	})
	_ = http.ListenAndServe(":8080", nil)
}
