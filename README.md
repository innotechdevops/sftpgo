# sftpgo

SFTP wrapper for Golang

## Install

```
go get github.com/innotechdevops/sftpgo
```

## How to use

``` golang
func New() sftpgo.SftpClient {
	config := sftpgo.Config{
		Host:     configuration.Config.Sftp.Report.Host,
		Username: configuration.Config.Sftp.Report.Username,
		Password: configuration.Config.Sftp.Report.Password,
		Port:     configuration.Config.Sftp.Report.Port,
	}

	csftp, err := sftpgo.NewClient(&config)
	if err != nil {
		log.Panic("New SFTP client", err.Error())
	}
	return csftp
}

client := New()
```