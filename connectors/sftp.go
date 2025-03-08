package connectors

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

type Sftp struct {
	host, user, password string
	port                 int
	sshConfig            *ssh.ClientConfig
	remoteDir            string
}

// Init the uploader
func (s *Sftp) Init() error {
	if err := s.setConfig(); err != nil {
		return err
	}

	rlog.Info(`SFTP client successfully initialized`)
	return nil
}

// PushFile to SFTP-Server
func (s *Sftp) Upload(fileName string) error {

	rlog.Info(`Start uploading file to remote ...`)

	// Establish SSH connection
	conn, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", s.host, s.port), s.sshConfig)

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

	remoteTargetDir := s.remoteDir + filepath.Base(fileName)
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

// setConfig for the remote ssh / sftp server
func (s *Sftp) setConfig() error {

	// set host config
	var err error
	s.port, err = strconv.Atoi(os.Getenv(`SFTP_PORT`))
	if err != nil {
		rlog.Infof(`Given remote server port is invalid, falling back to default SSH Port (22)`)
		s.port = 22
	}

	s.host = os.Getenv(`SFTP_HOST`)
	if s.host == `` {
		rlog.Infof(`No remote server hostname set, assuming 'localhost'`)
		s.host = `localhost`
	}

	s.remoteDir = os.Getenv(`TARGET_DIR`)
	s.remoteDir = strings.TrimSuffix(s.remoteDir, "/") // remove trailing slash if any
	if s.remoteDir == `` {
		s.remoteDir = `~`
		rlog.Infof(`No TARGET_DIR provided, assuming home-dir as remote directory`)
	}

	// set credentials
	s.user = os.Getenv(`SFTP_USER`)
	if s.user == `` {
		return fmt.Errorf(`user not set. Cannot continue`)
	}

	// set port or either to default 22 / SSH
	// Create ssh client configuration
	authMethod, err := s.getAuthMethod()
	if err != nil {
		return err
	}
	s.sshConfig = &ssh.ClientConfig{
		User:            s.user,
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
		s.sshConfig.HostKeyCallback = hostKeyCallback

	} else {
		rlog.Warnf(`KNOWN_HOSTS not set. Falling back to unchecked hostKeys`)
	}

	rlog.Infof("Using SFTP %s:%d", s.host, s.port)

	return nil
}

// getAuthMethod depending on configuration
func (s *Sftp) getAuthMethod() ([]ssh.AuthMethod, error) {

	auth := []ssh.AuthMethod{}

	// read pub ssh key if givena
	privKeyFile := os.Getenv(`SFTP_PRIV_KEY_FILE`)
	if privKeyFile != `` {
		authMethod, err := s.publicKeyAuth(privKeyFile)
		if err != nil {
			rlog.Warnf(`%v. Switching to password authentication ...`, err)
		} else {
			auth = append(auth, authMethod)
			rlog.Infof(`Will use private key authentication`)
			return auth, nil
		}
	}

	// use password if defined
	s.password = os.Getenv(`SFTP_PASSWORD`)
	if s.password == `` {
		return auth, errors.New(`no password and no private key available. No authentication possible`)
	}
	auth = append(auth, ssh.Password(s.password))
	rlog.Infof(`Will use password authentication`)
	return auth, nil
}

// publicKeyAuth loads Private Key and returns authentication method
func (s *Sftp) publicKeyAuth(keyPath string) (ssh.AuthMethod, error) {
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
