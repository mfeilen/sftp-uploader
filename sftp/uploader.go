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

var host, user, password string
var port int
var sshConfig *ssh.ClientConfig
var remoteDir string

// Init the uploader
func Init() error {
	if err := setSshConfig(); err != nil {
		return err
	}

	rlog.Info(`SFTP client successfully initialized`)
	return nil
}

// setSshConfig for the remote ssh / sftp server
func setSshConfig() error {

	// set host config
	var err error
	port, err = strconv.Atoi(os.Getenv(`SFTP_PORT`))
	if err != nil {
		rlog.Infof(`Given remote server port is invalid, falling back to default SSH Port (22)`)
		port = 22
	}

	host = os.Getenv(`SFTP_HOST`)
	if host == `` {
		rlog.Infof(`No remote server hostname set, assuming 'localhost'`)
		host = `localhost`
	}

	remoteDir = os.Getenv(`SFTP_TARGET_DIR`)
	remoteDir = strings.TrimSuffix(remoteDir, "/") // remove trailing slash if any
	if remoteDir == `` {
		rlog.Infof(`No SFTP_TARGET_DIR provided, assuming /uploads/ as remote directory`)
	}

	// set credentials
	user = os.Getenv(`SFTP_USER`)
	if user == `` {
		return fmt.Errorf(`user not set. Cannot continue`)
	}

	// set port or either to default 22 / SSH
	// Create ssh client configuration
	authMethod, err := getAuthMethod()
	if err != nil {
		return err
	}
	sshConfig = &ssh.ClientConfig{
		User:            user,
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
		sshConfig.HostKeyCallback = hostKeyCallback

	} else {
		rlog.Warnf(`KNOWN_HOSTS not set. Falling back to unchecked hostKeys`)
	}

	rlog.Infof("Using SFTP %s:%d", host, port)

	return nil
}

// getAuthMethod depending on configuration
func getAuthMethod() ([]ssh.AuthMethod, error) {

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
	password = os.Getenv(`SFTP_PASSWORD`)
	if password == `` {
		return auth, errors.New(`no password and no private key available. No authentication possible`)
	}
	auth = append(auth, ssh.Password(password))
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
func Upload(fileName string) error {

	rlog.Info(`Start uploading file to remote ...`)
	// Establish SSH connection
	conn, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", host, port), sshConfig)
	if err != nil {
		return fmt.Errorf("error creating SFTP-Connection, because: %w", err)
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

	remoteTargetDir := remoteDir + `/` + filepath.Base(fileName)
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
