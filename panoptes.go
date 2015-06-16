package panoptes

import (
	"fmt"
)

type Op uint32
type RawOp uint32

const (
	Create Op = 1 << iota // 1
	Modify                // 2
	Remove                // 4
	Rename                // 8
)

func (op Op) String() string {
	switch op {
	case Create:
		return "create"
	case Modify:
		return "modify"
	case Remove:
		return "remove"
	case Rename:
		return "rename"
	}
	return "unknown"
}

var (
	WatchedRootRemovedErr = fmt.Errorf("Watched root was removed")
)

type Event struct {
	Path    string
	OldPath string
	Op      Op
	IsDir   bool
}

func newEvent(path string, op Op, isDir bool) Event {
	return Event{Path: path, Op: op, IsDir: isDir}
}

func newRenameEvent(path string, oldPath string, isDir bool) Event {
	return Event{Path: path, Op: Rename, OldPath: oldPath, IsDir: isDir}
}

type Watcher interface {
	Events() <-chan Event
	Errors() <-chan error
	Close() error
}
