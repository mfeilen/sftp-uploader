package watcher

type Uploader interface {
	Init() error
	Upload(fileName string) error
}
