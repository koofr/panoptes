package panoptes

import "fmt"

type Op uint32
type RawOp uint32

const (
	Create Op = 1 << iota
	Write
	Remove
	Rename
)

var (
	WatchedRootRemovedErr = fmt.Errorf("Watched root was removed")
)

type Event struct {
	Path    string
	OldPath string
	Op      Op
}

func newEvent(path string, op Op) Event {
	return Event{Path: path, Op: op}
}

func newRenameEvent(path string, oldPath string) Event {
	return Event{Path: path, Op: Rename, OldPath: oldPath}
}

type Watcher interface {
	Events() <-chan Event
	Errors() <-chan error
	WatchedPaths() []string
	IgnoredPaths() []string
	Close() error
}
