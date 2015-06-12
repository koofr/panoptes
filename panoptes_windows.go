// +build windows

package panoptes

import (
	"os"
	"sync"
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

type WinWatcher struct {
	watchedPath string
	events      chan Event
	errors      chan error
	movedTo     chan string
	createdLock sync.RWMutex
	created     map[string]chan error
	raw         *fsnotify.Watcher
	isClosed    bool
	quitCh      chan error
}

func NewWatcher(path string) (w *WinWatcher, err error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return
	}

	watcher.Recursive = true

	w = &WinWatcher{
		watchedPath: path,
		events:      make(chan Event),
		errors:      make(chan error),
		movedTo:     make(chan string),
		created:     make(map[string]chan error),
		raw:         watcher,
		quitCh:      make(chan error),
	}

	go w.translateEvents()
	go w.translateErrors()

	w.raw.Add(path)

	return
}

func (w *WinWatcher) translateEvents() {
	for event := range w.raw.Events {
		switch {
		case event.RawOp&IN_DELETE == IN_DELETE:
			w.sendEvent(newEvent(event.Name, Remove))
		case event.RawOp&IN_DELETE_SELF == IN_DELETE_SELF:
			if event.Name == w.watchedPath {
				w.errors <- WatchedRootRemovedErr
			} else {
				continue
			}
		case event.RawOp&IN_CREATE == IN_CREATE:
			if info, err := os.Stat(event.Name); err == nil {
				if info.IsDir() {
					w.sendEvent(newEvent(event.Name, Create))
				} else {
					w.createdLock.Lock()
					w.created[event.Name] = make(chan error, 1)
					w.created[event.Name] <- nil
					w.createdLock.Unlock()
				}
			}

		case event.RawOp&IN_MODIFY == IN_MODIFY:
			w.createdLock.RLock()
			select {
			case <-w.created[event.Name]:
				w.sendEvent(newEvent(event.Name, Create))
			default:
				w.sendEvent(newEvent(event.Name, Modify))
			}
			w.createdLock.RUnlock()

		case event.RawOp&IN_MOVED_FROM == IN_MOVED_FROM:
			go func(event fsnotify.Event) {
				select {
				case newPth := <-w.movedTo:
					w.sendEvent(newRenameEvent(newPth, event.Name))
				case <-time.After(500 * time.Millisecond):
					w.sendEvent(newEvent(event.Name, Remove))
				}
			}(event)

		case event.RawOp&IN_MOVED_TO == IN_MOVED_TO:
			go func(event fsnotify.Event) {
				select {
				case w.movedTo <- event.Name:
				default:
					w.sendEvent(newEvent(event.Name, Create))
				}
			}(event)
		}
	}
}

func (w *WinWatcher) Events() <-chan Event {
	return w.events
}

func (w *WinWatcher) Errors() <-chan error {
	return w.errors
}

func (w *WinWatcher) translateErrors() {
	for err := range w.raw.Errors {
		w.errors <- err
	}
}

func (w *WinWatcher) sendEvent(e Event) {
	w.events <- e
}

func (w *WinWatcher) Close() error {
	if w.isClosed {
		return nil
	}
	w.isClosed = true
	close(w.quitCh)

	err := w.raw.Close()
	close(w.events)
	close(w.errors)
	return err
}
