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
		movedTo:      make(map[uint32]chan string),
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
			w.movedTo[event.EventID] = make(chan string, 1)

			go func(event fsnotify.Event) {
				select {
				case newPth := <-w.movedTo[event.EventID]:
					w.sendEvent(newRenameEvent(newPth, event.Name))
				case <-time.After(500 * time.Millisecond):
					w.sendEvent(newEvent(event.Name, Remove))
				}
			}(event)

		case event.RawOp&syscall.IN_MOVED_TO == syscall.IN_MOVED_TO:

			go func(event fsnotify.Event) {
				select {
				case w.movedTo[event.EventID] <- event.Name:
				default:
					w.sendEvent(newEvent(event.Name, Create))
				}
			}(event)
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
