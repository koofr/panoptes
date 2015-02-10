// +build windows

package panoptes

import (
	"time"

	"github.com/koofr/fsnotify"
)

const (
	IN_ACCESS        = 0x1
	IN_MODIFY        = 0x2
	IN_ATTRIB        = 0x4
	IN_CLOSE_WRITE   = 0x8
	IN_CLOSE_NOWRITE = 0x10
	IN_OPEN          = 0x20
	IN_MOVED_FROM    = 0x40
	IN_MOVED_TO      = 0x80
	IN_CREATE        = 0x100
	IN_DELETE        = 0x200
	IN_DELETE_SELF   = 0x400
	IN_CLOSE         = IN_CLOSE_NOWRITE | IN_CLOSE_WRITE
	IN_MOVE          = IN_MOVED_FROM | IN_MOVED_TO
	IN_ISDIR         = 0x40000000
	IN_ONESHOT       = 0x80000000
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
		w.raw.Add(pth)
	}
	return
}

func (w *LinWinWatcher) translateEvents() {
	for event := range w.raw.Events {
		switch {
		case event.RawOp&IN_DELETE == IN_DELETE:
			w.sendEvent(newEvent(event.Name, Remove))
		case event.RawOp&IN_DELETE_SELF == IN_DELETE_SELF:
			if w.isWatchedRoot(event.Name) {
				w.errors <- WatchedRootRemovedErr
			} else {
				continue
			}
		case event.RawOp&IN_CREATE == IN_CREATE:
			w.sendEvent(newEvent(event.Name, Create))
		case event.RawOp&IN_MODIFY == IN_MODIFY:
			w.sendEvent(newEvent(event.Name, Write))
		case event.RawOp&IN_MOVED_FROM == IN_MOVED_FROM:
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
		case event.RawOp&IN_MOVED_TO == IN_MOVED_TO:
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
		}
	}
}
