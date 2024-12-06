package main

import (
	"os"
	"sftp-uploader/watcher"

	"github.com/joho/godotenv"
	"github.com/romana/rlog"
)

func main() {

	// set log configuration
	_, err := os.Stat(`.env.log`)
	if os.IsExist(err) {
		rlog.SetConfFile(`.env.log`)
	}

	// set app configration
	err = godotenv.Load(`.env`)
	if err != nil {
		rlog.Errorf(`Could not read .env configuration, because %v`, err)
		return
	}

	// start watcher / uploader
	if err := watcher.Init(); err != nil {
		rlog.Errorf(`Initialisation failed, because %v`, err)
		return
	}

	if err := watcher.Start(); err != nil {
		rlog.Error(err)
	}

	rlog.Info(`Shutting down!`)
}
