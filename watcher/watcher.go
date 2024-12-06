package watcher

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sftp-uploader/sftp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/romana/rlog"
)

const defaultPollIntervall = 2
const defaultPollRetries = 100

var fileChangeInterval time.Duration
var maxPollRetries int
var deleteAfterUpload bool
var watchDir string
var archiveDir string
var failedDir string
var shutDownAfterXerrors int

func Start() error {

	var wg sync.WaitGroup

	if err := sftp.Init(); err != nil {
		return err
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf(`error creating watcher Watchers because: %v`, err)
	}
	defer watcher.Close()

	if err := watcher.Add(watchDir); err != nil {
		return fmt.Errorf(`error adding watch directory, because: %v`, err)
	}
	rlog.Infof(`Will watch directory %s for new files`, watchDir)

	// cancel signal
	signalChan := make(chan os.Signal, 1)
	done := make(chan bool, 1)

	// catch SIGINT (Strg+C) SIGTERM and push it into signalChan
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	// init uploader
	errCounter := 0

	wg.Add(1)
	go func() {
		defer wg.Done()
		rlog.Info(`Succesfullly started. Waiting for new files ...`)
		for {
			select {
			case <-signalChan:
				done <- true
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Create == fsnotify.Create {
					rlog.Infof("New file found: %s. Checking if it is still changing ...", event.Name)

					// check if upload is complete
					go handleNewFile(event.Name, watcher.Errors)
				}
			case err := <-watcher.Errors:
				if err != nil {
					rlog.Errorf(`SFTP server problem: %v`, err)
					if shutDownAfterXerrors > 0 {
						errCounter++
						if errCounter == shutDownAfterXerrors {
							rlog.Error("Too many errors occured. Giving up")
							return
						}
					}
				}
			}
		}
	}()
	wg.Wait()

	return nil
}

// Init watcher configuration
func Init() error {

	shutDownAfterXerrors = func() int {
		if os.Getenv(`SHUT_DOWN_AFTER_ERRORS`) == `` {
			return 0
		}

		amount, err := strconv.Atoi(os.Getenv(`SHUT_DOWN_AFTER_ERRORS`))
		if err != nil {
			rlog.Warn(`Given shutdown error amountis invalid, disabling it`)
			return 0
		}

		return amount
	}()

	watchDir = os.Getenv(`WATCH_DIR`)
	if !directoryExists(watchDir) {
		return fmt.Errorf(`watch directory %s not set properly`, watchDir)
	}

	failedDir = os.Getenv(`FAILED_DIR`)
	if failedDir != `` && !directoryExists(failedDir) {
		return fmt.Errorf(`failed file directory %s not set properly`, failedDir)
	}

	archiveDir = os.Getenv(`ARCHIVE_DIR`)
	if archiveDir != `` && !directoryExists(archiveDir) {
		return fmt.Errorf(`archive directory %s not set properly`, archiveDir)
	}

	fileChangeInterval = func() time.Duration {
		if os.Getenv(`WATCH_FILE_CHANGE_INTERVAL`) == `` {
			return defaultPollIntervall * time.Second
		}

		interval, err := strconv.Atoi(os.Getenv(`WATCH_FILE_CHANGE_INTERVAL`))
		if err != nil {
			rlog.Warnf(`Given watch wait interval is invalid, falling back to %d seconds`, defaultPollIntervall)
			return defaultPollIntervall * time.Second
		}
		return time.Duration(interval) * time.Second

	}()

	maxPollRetries = func() int {
		if os.Getenv(`WATCH_FILE_CHANGE_MAX_TIME`) == `` {
			return defaultPollRetries
		}
		amount, err := strconv.Atoi(os.Getenv(`WATCH_FILE_CHANGE_MAX_TIME`))

		if err != nil {
			rlog.Warnf(`Given watch retry number is invalid, falling back to %d retry`, defaultPollRetries)
		}
		return amount
	}()

	deleteAfterUpload = strings.ToLower(os.Getenv(`DELETE_FILE_AFTER`)) == `true`

	return nil
}

// directoryExists used in configuration
func directoryExists(dirPath string) bool {
	info, err := os.Stat(dirPath)
	if err != nil {

		if os.IsNotExist(err) {
			return false
		}

		// Unexpected errors are logged too
		rlog.Error(err)
		return false
	}

	return info.IsDir()
}

// handleNewFile checks if the file upload was completed. Then it is mailed
func handleNewFile(fileName string, errChan chan error) {

	retryCount := 0
	for {
		// Check if file is still changing / uploadings somehwere
		if isFileComplete(fileName) {
			rlog.Infof("File %s seems to be complete.", fileName)

			err := sftp.Upload(fileName)
			if err != nil {
				moveFailedFile(fileName)
				errChan <- err
				break
			}

			archiveFile(fileName)
			break
		}

		if retryCount > maxPollRetries {
			rlog.Infof("File %s is continuously changing and will be ignored now. You may adjust your file watcher configuration to prevent this.", fileName)
			break
		}
		time.Sleep(1 * time.Second)

		retryCount++
	}
}

// moveFailedFile to a separate folder, if defined
func moveFailedFile(fileName string) {

	failedDir := os.Getenv(`FAILED_DIR`)
	if failedDir == `` {
		return
	}

	baseFileName := filepath.Base(fileName)
	err := os.Rename(fileName, failedDir+`/`+baseFileName)
	if err != nil {
		rlog.Warnf(`could not archive file %s, because %v. It stays in the folder watched and will be ignored.`, baseFileName, err)
	}
}

// archive file if configured
func archiveFile(fileName string) error {

	// either move to another folder after upload
	if archiveDir != `` {
		archiveFileName := archiveDir + `/` + filepath.Base(fileName)
		if err := os.Rename(fileName, archiveFileName); err != nil {
			return fmt.Errorf(`error moving file '%s' to %s, because %v`, fileName, archiveFileName, err)
		}
		rlog.Infof(`File %s was archived to %s`, filepath.Base(fileName), archiveFileName)
		return nil
	}

	// or delete after successful upload
	if deleteAfterUpload {
		if err := os.Remove(fileName); err != nil {
			return fmt.Errorf(`error deleting file '%s' after upload`, fileName)
		}
	}

	return nil
}

// isFileComplete checks if the file upload was completed
func isFileComplete(filePath string) bool {
	file, err := os.Open(filePath)
	if err != nil {
		return false
	}
	defer file.Close()

	fileStat, err := file.Stat() // check file size
	if err != nil {
		return false
	}

	// wait a little to check if file size changes
	time.Sleep(fileChangeInterval)
	stat2, err := os.Stat(filePath)
	if err != nil {
		return false
	}

	return fileStat.Size() == stat2.Size()
}
