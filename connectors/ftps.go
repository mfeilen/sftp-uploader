package connectors

import (
	"crypto/tls"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jlaffaye/ftp"
	"github.com/romana/rlog"
)

type Ftps struct {
	host, user, password string
	port                 int
	tlsConfig            *tls.Config
	remoteDir            string
}

// Init the FTPs server configuration
func (f *Ftps) Init() error {
	if err := f.setConfig(); err != nil {
		return err
	}

	rlog.Info(`FTPs client successfully initialized`)
	return nil

}

// PushFile to FTPS-Server
func (f *Ftps) Upload(fileName string) error {

	rlog.Info(`Start uploading file to remote ...`)

	// open upload file before connect
	uploadFile, err := os.Open(fileName)
	if err != nil {
		return fmt.Errorf("cannot open file %s, because %v", fileName, err)
	}
	defer uploadFile.Close()

	localStat, err := uploadFile.Stat()
	if err != nil {
		return fmt.Errorf("error stat'ing local file: %v", err)
	}

	// Connect to the FTP server using TLS encryption.
	ftpdialoption := ftp.DialWithExplicitTLS(f.tlsConfig)

	conn, err := ftp.Dial(
		fmt.Sprintf("%s:%d", f.host, f.port),
		ftpdialoption,
		//		ftp.DialWithDebugOutput(os.Stdout), // deep debug
	)

	if err != nil {
		return fmt.Errorf("error creating FTPs connection, because: %w", err)
	}
	defer conn.Quit()

	// login
	err = conn.Login(f.user, f.password)
	if err != nil {
		return fmt.Errorf("error authenticating against FTPs server, because : %w", err)
	}

	// check upload dir ok
	if err := f.ensureDirExists(conn); err != nil {
		return err
	}

	// now upload
	remoteTargetDir := f.remoteDir + filepath.Base(fileName)
	if err := conn.Stor(remoteTargetDir, uploadFile); err != nil {
		return fmt.Errorf("error uploading file %s new FTPs client, because : %w", filepath.Base(fileName), err)
	}

	remoteSize, err := conn.FileSize(remoteTargetDir)
	if err != nil {
		return fmt.Errorf("error uploading file %s new FTPs client, because : %w", filepath.Base(fileName), err)
	}

	if localStat.Size() != remoteSize {
		return fmt.Errorf("file size mismatch after upload: local %d bytes, remote %d bytes", localStat.Size(), remoteSize)
	}

	rlog.Infof(`File %s was successfully uploaded to %s.`, filepath.Base(fileName), f.host)

	return nil
}

// setConfig for remote SFTP
func (f *Ftps) setConfig() error {

	// set host config
	var err error
	f.port, err = strconv.Atoi(os.Getenv(`FTPS_PORT`))
	if err != nil {
		rlog.Infof(`Given remote server port is invalid, falling back to default SSH Port (22)`)
		f.port = 22
	}

	f.host = os.Getenv(`FTPS_HOST`)
	if f.host == `` {
		rlog.Infof(`No remote server hostname set, assuming 'localhost'`)
		f.host = `localhost`
	}

	f.remoteDir = os.Getenv(`TARGET_DIR`)
	f.remoteDir = strings.TrimSuffix(f.remoteDir, "/") + `/` // remove trailing slash if any
	if f.remoteDir == `` {
		f.remoteDir = `/`
		rlog.Infof(`No TARGET_DIR provided, assuming / as remote directory`)
	}

	// set credentials
	f.user = os.Getenv(`FTPS_USER`)
	if f.user == `` {
		return fmt.Errorf(`user not set. Cannot continue`)
	}

	f.password = os.Getenv(`FTPS_PASSWORD`)
	if f.password == `` {
		return fmt.Errorf(`no password set in FTPS_PASSWORD. No authentication possible`)
	}

	f.tlsConfig = &tls.Config{
		// Enable TLS 1.2.
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS12,
		// Cert not implemented yet
	}

	rlog.Infof("Using FTPS %s:%d", f.host, f.port)

	return nil
}

// Prüft, ob ein Verzeichnis existiert, und wechselt hinein
func (f *Ftps) ensureDirExists(conn *ftp.ServerConn) error {

	// Change to upload dir
	if err := conn.ChangeDir(f.remoteDir); err == nil {
		rlog.Infof(`Changed to upload directory %s`, f.remoteDir)
		return nil
	}

	// If not exists, try to create
	if err := conn.MakeDir(f.remoteDir); err != nil {
		return fmt.Errorf("error creating upload directory %s, because: ", f.remoteDir, err)
	}

	// Nach Erstellung nochmal hineinwechseln
	if err := conn.ChangeDir(f.remoteDir); err != nil {
		return fmt.Errorf("error changing into created directory %s, because: ", f.remoteDir, err)
	}

	return nil
}
