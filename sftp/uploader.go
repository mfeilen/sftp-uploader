package sftp

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pkg/sftp"
	"github.com/romana/rlog"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

type Uploader struct {
	host, user, password string
	port                 int
	sshConfig            *ssh.ClientConfig
	remoteDir            string
}

// Init the uploader
func (u *Uploader) Init() error {
	if err := u.setConfig(); err != nil {
		return err
	}

	rlog.Info(`SFTP client successfully initialized`)
	return nil
}

// setConfig for the remote ssh / sftp server
func (u *Uploader) setConfig() error {

	// set host config
	var err error
	u.port, err = strconv.Atoi(os.Getenv(`SFTP_PORT`))
	if err != nil {
		rlog.Infof(`Given remote server port is invalid, falling back to default SSH Port (22)`)
		u.port = 22
	}

	u.host = os.Getenv(`SFTP_HOST`)
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
	u.user = os.Getenv(`SFTP_USER`)
	if u.user == `` {
		return fmt.Errorf(`user not set. Cannot continue`)
	}

	// set port or either to default 22 / SSH
	// Create ssh client configuration
	authMethod, err := u.getAuthMethod()
	if err != nil {
		return err
	}
	u.sshConfig = &ssh.ClientConfig{
		User:            u.user,
		Auth:            authMethod,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	// hostKey / fingerprint check?
	knownHostsFile := os.Getenv(`KNOWN_HOSTS`)
	if knownHostsFile != `` {
		hostKeyCallback, err := knownhosts.New(knownHostsFile)
		if err != nil {
			rlog.Warnf(`Known Hosts file not found in %s, because %v.`, knownHostsFile, err)
		}
		u.sshConfig.HostKeyCallback = hostKeyCallback

	} else {
		rlog.Warnf(`KNOWN_HOSTS not set. Falling back to unchecked hostKeys`)
	}

	rlog.Infof("Using SFTP %s:%d", u.host, u.port)

	return nil
}

// getAuthMethod depending on configuration
func (u *Uploader) getAuthMethod() ([]ssh.AuthMethod, error) {

	auth := []ssh.AuthMethod{}

	// read pub ssh key if givena
	privKeyFile := os.Getenv(`SFTP_PRIV_KEY_FILE`)
	if privKeyFile != `` {
		authMethod, err := publicKeyAuth(privKeyFile)
		if err != nil {
			rlog.Warnf(`%v. Switching to password authentication ...`, err)
		} else {
			auth = append(auth, authMethod)
			rlog.Infof(`Will use private key authentication`)
			return auth, nil
		}
	}

	// use password if defined
	u.password = os.Getenv(`SFTP_PASSWORD`)
	if u.password == `` {
		return auth, errors.New(`no password and no private key available. No authentication possible`)
	}
	auth = append(auth, ssh.Password(u.password))
	rlog.Infof(`Will use password authentication`)
	return auth, nil
}

// publicKeyAuth loads Private Key and returns authentication method
func publicKeyAuth(keyPath string) (ssh.AuthMethod, error) {
	key, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("error reading private key file because: %v", err)
	}

	// Ready private Key
	var signer ssh.Signer
	if os.Getenv(`SFTP_PRIV_KEY_PASSWORD`) != `` { // with ssh key pw
		signer, err = ssh.ParsePrivateKeyWithPassphrase(key, []byte(os.Getenv(`SFTP_PRIV_KEY_PASSWORD`)))
	} else {
		signer, err = ssh.ParsePrivateKey(key)
	}

	if err != nil {
		return nil, fmt.Errorf("error processing private key, because: %v", err)
	}

	return ssh.PublicKeys(signer), nil
}

// PushFile to SFTP-Server
func (u *Uploader) Upload(fileName string) error {

	rlog.Info(`Start uploading file to remote ...`)

	// Establish SSH connection
	conn, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", u.host, u.port), u.sshConfig)
	if err != nil {
		return fmt.Errorf("error creating SFTP connection, because: %w", err)
	}
	defer conn.Close()

	// Create SFTP-Client
	client, err := sftp.NewClient(conn)
	if err != nil {
		return fmt.Errorf("error creating new SFTP client, because : %w", err)
	}
	defer client.Close()

	// Upload file
	srcFile, err := os.Open(fileName)
	if err != nil {
		return fmt.Errorf("error reading file, because : %w", err)
	}
	defer srcFile.Close()

	remoteTargetDir := u.remoteDir + `/` + filepath.Base(fileName)
	dstFile, err := client.Create(remoteTargetDir)
	if err != nil {
		return fmt.Errorf("error creating file on SFTP server in %s because: %w", remoteTargetDir, err)
	}
	defer dstFile.Close()

	_, err = dstFile.ReadFrom(srcFile)
	if err != nil {
		return fmt.Errorf("error writing to SFTP server, because : %w", err)
	}

	rlog.Infof(`File %s was successfully uploaded to %s`, filepath.Base(fileName), remoteTargetDir)

	return nil
}
