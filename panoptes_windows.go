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
		events:      make(chan Event, 1024),
		errors:      make(chan error),
		movedTo:     make(chan string),
		created:     make(map[string]chan error),
		raw:         watcher,
		quitCh:      make(chan error),
	}

	go w.translateEvents()

	w.raw.Add(path)

	return
}

func isDir(e fsnotify.Event) bool {
	return e.RawOp&IN_ISDIR == IN_ISDIR
}

func (w *WinWatcher) translateEvents() {

	defer func() {
		close(w.errors)
		close(w.events)
	}()

	for {
		select {
		case <-w.quitCh:
			return
		case err, ok := <-w.raw.Errors:
			if !ok {
				return
			}
			w.errors <- err

		case event, ok := <-w.raw.Events:
			if !ok {
				return
			}

			switch {
			case event.RawOp&IN_DELETE == IN_DELETE:
				w.events <- newEvent(event.Name, Remove, isDir(event))
			case event.RawOp&IN_DELETE_SELF == IN_DELETE_SELF:
				if event.Name == w.watchedPath {
					w.errors <- WatchedRootRemovedErr
				} else {
					continue
				}
			case event.RawOp&IN_CREATE == IN_CREATE:
				if info, err := os.Stat(event.Name); err == nil {
					if info.IsDir() {
						w.events <- newEvent(event.Name, Create, isDir(event))
					} else {
						w.createdLock.Lock()
						w.created[event.Name] = make(chan error, 1)
						w.created[event.Name] <- nil
						time.AfterFunc(3*time.Second, func() {
							w.createdLock.Lock()
							defer w.createdLock.Unlock()
							select {
							case <-w.created[event.Name]:
								w.events <- newEvent(event.Name, Create, isDir(event))
							default:
							}
						})
						w.createdLock.Unlock()
					}
				}

			case event.RawOp&IN_MODIFY == IN_MODIFY:
				w.createdLock.RLock()
				select {
				case <-w.created[event.Name]:
					w.events <- newEvent(event.Name, Create, isDir(event))
				default:
					w.events <- newEvent(event.Name, Modify, isDir(event))
				}
				w.createdLock.RUnlock()

			case event.RawOp&IN_MOVED_FROM == IN_MOVED_FROM:
				go func(event fsnotify.Event) {
					select {
					case newPth := <-w.movedTo:
						w.events <- newRenameEvent(newPth, event.Name, isDir(event))
					case <-time.After(500 * time.Millisecond):
						w.events <- newEvent(event.Name, Remove, isDir(event))
					}
				}(event)

			case event.RawOp&IN_MOVED_TO == IN_MOVED_TO:
				go func(event fsnotify.Event) {
					select {
					case w.movedTo <- event.Name:
					default:
						w.events <- newEvent(event.Name, Create, isDir(event))
					}
				}(event)
			}
		}
	}
}

func (w *WinWatcher) Events() <-chan Event {
	return w.events
}

func (w *WinWatcher) Errors() <-chan error {
	return w.errors
}

func (w *WinWatcher) Close() error {
	if w.isClosed {
		return nil
	}
	w.isClosed = true
	close(w.quitCh)
	err := w.raw.Close()
	return err
}
