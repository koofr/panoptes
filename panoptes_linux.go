// +build linux

package panoptes

import (
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/koofr/fsnotify"
)

type LinuxWatcher struct {
	watchedPath string
	events      chan Event
	errors      chan error
	movedToLock sync.RWMutex
	movedTo     map[uint32]chan string
	createdLock sync.RWMutex
	created     map[string]chan error
	raw         *fsnotify.Watcher
	quitCh      chan error
	isClosed    bool
}

func NewWatcher(path string) (w *LinuxWatcher, err error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return
	}

	w = &LinuxWatcher{
		watchedPath: path,
		events:      make(chan Event),
		errors:      make(chan error),
		movedTo:     make(map[uint32]chan string),
		created:     make(map[string]chan error),
		quitCh:      make(chan error),
		raw:         watcher,
	}

	go w.translateEvents()
	go w.translateErrors()

	if err := w.recursiveAdd(path); err != nil {
		return nil, err
	}
	return
}

func (w *LinuxWatcher) translateEvents() {
	for {
		select {
		case <-w.quitCh:
			return
		case event, ok := <-w.raw.Events:
			if !ok {
				return
			}
			switch {
			case event.RawOp&syscall.IN_DELETE == syscall.IN_DELETE:
				w.sendEvent(newEvent(event.Name, Remove))
			case event.RawOp&syscall.IN_DELETE_SELF == syscall.IN_DELETE_SELF:
				if w.watchedPath == event.Name {
					w.errors <- WatchedRootRemovedErr
				} else {
					continue
				}
			case event.RawOp&syscall.IN_CREATE == syscall.IN_CREATE:
				if event.RawOp&syscall.IN_ISDIR == syscall.IN_ISDIR {
					w.recursiveAdd(event.Name)
					w.sendEvent(newEvent(event.Name, Create))
				} else {
					w.createdLock.Lock()
					w.created[event.Name] = make(chan error, 1)
					w.created[event.Name] <- nil
					w.createdLock.Unlock()
				}
			case event.RawOp&syscall.IN_CLOSE_WRITE == syscall.IN_CLOSE_WRITE:
				w.createdLock.RLock()
				select {
				case <-w.created[event.Name]:
					w.sendEvent(newEvent(event.Name, Create))
				default:
					w.sendEvent(newEvent(event.Name, Modify))
				}
				w.createdLock.RUnlock()

			case event.RawOp&syscall.IN_MOVED_FROM == syscall.IN_MOVED_FROM:
				w.movedToLock.Lock()
				w.movedTo[event.EventID] = make(chan string, 1)
				w.movedToLock.Unlock()

				go func(event fsnotify.Event) {
					w.movedToLock.RLock()
					select {
					case newPth := <-w.movedTo[event.EventID]:
						w.sendEvent(newRenameEvent(newPth, event.Name))
					case <-time.After(500 * time.Millisecond):
						w.sendEvent(newEvent(event.Name, Remove))
					}
					w.movedToLock.RUnlock()
				}(event)

			case event.RawOp&syscall.IN_MOVED_TO == syscall.IN_MOVED_TO:

				w.movedToLock.RLock()
				ch := w.movedTo[event.EventID]
				w.movedToLock.RUnlock()

				go func(event fsnotify.Event) {
					select {
					case ch <- event.Name:
					default:
						w.sendEvent(newEvent(event.Name, Create))
					}
				}(event)
			}
		}
	}
}
func (w *LinuxWatcher) recursiveAdd(root string) error {

	err := filepath.Walk(root, func(pth string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			w.raw.Add(pth)
		}

		return nil
	})
	return err
}

func (w *LinuxWatcher) Events() <-chan Event {
	return w.events
}

func (w *LinuxWatcher) Errors() <-chan error {
	return w.errors
}

func (w *LinuxWatcher) translateErrors() {
	for {
		select {
		case <-w.quitCh:
			return
		case err, ok := <-w.raw.Errors:
			if ok {
				w.errors <- err
			}
		}
	}
}

func (w *LinuxWatcher) sendEvent(e Event) {
	select {
	case <-w.quitCh:
		return
	default:
		w.events <- e
	}
}

func (w *LinuxWatcher) Close() error {
	if w.isClosed {
		return nil
	}
	w.isClosed = true
	err := w.raw.Close()
	close(w.quitCh)
	close(w.events)
	close(w.errors)
	return err
}
