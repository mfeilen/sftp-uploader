package ftps

import (
	"crypto/tls"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jlaffaye/ftp"
	"github.com/romana/rlog"
)

type Uploader struct {
	host, user, password string
	port                 int
	tlsConfig            *tls.Config
	remoteDir            string
}

// Init the FTPs server configuration
func (u *Uploader) Init() error {
	if err := u.setConfig(); err != nil {
		return err
	}

	rlog.Info(`FTPs client successfully initialized`)
	return nil

}

// setConfig for remote SFTP
func (u *Uploader) setConfig() error {

	// set host config
	var err error
	u.port, err = strconv.Atoi(os.Getenv(`FTPS_PORT`))
	if err != nil {
		rlog.Infof(`Given remote server port is invalid, falling back to default SSH Port (22)`)
		u.port = 22
	}

	u.host = os.Getenv(`FTPS_HOST`)
	if u.host == `` {
		rlog.Infof(`No remote server hostname set, assuming 'localhost'`)
		u.host = `localhost`
	}

	u.remoteDir = os.Getenv(`TARGET_DIR`)
	u.remoteDir = strings.TrimSuffix(u.remoteDir, "/") // remove trailing slash if any
	if u.remoteDir == `` {
		rlog.Infof(`No TARGET_DIR provided, assuming / as remote directory`)
	}

	// set credentials
	u.user = os.Getenv(`FTPS_USER`)
	if u.user == `` {
		return fmt.Errorf(`user not set. Cannot continue`)
	}

	u.tlsConfig = &tls.Config{
		// Enable TLS 1.2.
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS12,
		// Cert not implemented yet
	}

	rlog.Infof("Using FTPS %s:%d", u.host, u.port)

	return nil
}

// PushFile to FTPS-Server
func (u *Uploader) Upload(fileName string) error {

	rlog.Info(`Start uploading file to remote ...`)

	// Connect to the FTP server using TLS encryption.
	ftpdialoption := ftp.DialWithExplicitTLS(u.tlsConfig)

	conn, err := ftp.Dial(u.host, ftpdialoption)
	if err != nil {
		return fmt.Errorf("error creating FTPs connection, because: %w", err)
	}
	defer conn.Quit()

	// login
	err = conn.Login(u.user, u.password)
	if err != nil {
		return fmt.Errorf("error authenticating against FTPs server, because : %w", err)
	}

	if err := u.ensureDirExists(conn); err != nil {
		return err
	}

	remoteFile := u.remoteDir + "/" + filepath.Base(fileName)
	r, err := conn.Retr(remoteFile)
	if err != nil {
		return fmt.Errorf("error uploading file %s new SFTP client, because : %w", filepath.Base(fileName), err)
	}

	r.Close()

	buf, _ := io.ReadAll(r)

	rlog.Infof(`File %s was successfully uploaded to %s. Server said: %s`, filepath.Base(fileName), u.host, string(buf))

	return nil
}

// Prüft, ob ein Verzeichnis existiert, und wechselt hinein
func (u *Uploader) ensureDirExists(conn *ftp.ServerConn) error {

	// Change to upload dir
	if err := conn.ChangeDir(u.remoteDir); err == nil {
		rlog.Infof(`Changed to upload directory %s`, u.remoteDir)
		return nil
	}

	// If not exists, try to create
	if err := conn.MakeDir(u.remoteDir); err != nil {
		return fmt.Errorf("error creating upload directory %s, because: ", u.remoteDir, err)
	}

	// Nach Erstellung nochmal hineinwechseln
	if err := conn.ChangeDir(u.remoteDir); err != nil {
		return fmt.Errorf("error changing into created directory %s, because: ", u.remoteDir, err)
	}

	return nil
}
