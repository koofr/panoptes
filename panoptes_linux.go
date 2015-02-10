// +build linux

package panoptes

import (
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/koofr/fsnotify"
)

func NewWatcher(paths []string, ignorePaths []string) (w *LinWinWatcher, err error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return
	}

	w = &LinWinWatcher{
		watchedPaths: paths,
		ignoredPaths: ignorePaths,
		events:       make(chan Event),
		errors:       make(chan error),
		renames:      make(map[uint32]string),
		raw:          watcher,
	}

	go w.translateEvents()
	go w.translateErrors()

	for _, pth := range paths {
		if err := w.recursiveAdd(pth); err != nil {
			return nil, err
		}
	}
	return
}

func (w *LinWinWatcher) translateEvents() {
	for event := range w.raw.Events {
		switch {
		case event.RawOp&syscall.IN_DELETE == syscall.IN_DELETE:
			w.sendEvent(newEvent(event.Name, Remove))
		case event.RawOp&syscall.IN_DELETE_SELF == syscall.IN_DELETE_SELF:
			if w.isWatchedRoot(event.Name) {
				w.errors <- WatchedRootRemovedErr
			} else {
				continue
			}
		case event.RawOp&syscall.IN_CREATE == syscall.IN_CREATE:
			if event.RawOp&syscall.IN_ISDIR == syscall.IN_ISDIR {
				w.recursiveAdd(event.Name)
			}
			w.sendEvent(newEvent(event.Name, Create))
		case event.RawOp&syscall.IN_MODIFY == syscall.IN_MODIFY:
			continue
		case event.RawOp&syscall.IN_CLOSE_WRITE == syscall.IN_CLOSE_WRITE:
			w.sendEvent(newEvent(event.Name, Write))
		case event.RawOp&syscall.IN_MOVED_FROM == syscall.IN_MOVED_FROM:
			w.renames[event.EventID] = event.Name
			time.AfterFunc(500*time.Millisecond, func() {
				w.renamesLock.Lock()
				defer w.renamesLock.Unlock()
				_, has := w.renames[event.EventID]
				if has {
					// file was moved out of watched folder
					w.sendEvent(newEvent(event.Name, Remove))
					delete(w.renames, event.EventID)
				}
			})
		case event.RawOp&syscall.IN_MOVED_TO == syscall.IN_MOVED_TO:
			w.renamesLock.Lock()
			defer w.renamesLock.Unlock()
			oldName, has := w.renames[event.EventID]
			if has {
				// file inside watched directory was renamed
				delete(w.renames, event.EventID)
				w.sendEvent(newRenameEvent(event.Name, oldName))
			} else {
				// file was moved into watched directory
				w.sendEvent(newEvent(event.Name, Create))
			}
			w.sendEvent(newEvent(event.Name, Write))
		}
	}
}

func (w *LinWinWatcher) recursiveAdd(root string) error {

	err := filepath.Walk(root, func(pth string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() && !w.isIgnoredPath(pth) {
			w.raw.Add(pth)
		}

		return nil
	})
	return err
}
