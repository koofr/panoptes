// +build linux

package panoptes

import (
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/koofr/fsnotify"
)

type LinuxWatcher struct {
	watchedPath string
	events      chan Event
	errors      chan error
	movedTo     map[uint32]chan string
	created     map[string]chan error
	raw         *fsnotify.Watcher
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
	for event := range w.raw.Events {
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
				w.created[event.Name] = make(chan error, 1)
				w.created[event.Name] <- nil
			}
		case event.RawOp&syscall.IN_CLOSE_WRITE == syscall.IN_CLOSE_WRITE:
			select {
			case <-w.created[event.Name]:
				w.sendEvent(newEvent(event.Name, Create))
			default:
				w.sendEvent(newEvent(event.Name, Modify))
			}

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
	for err := range w.raw.Errors {
		w.errors <- err
	}
}

func (w *LinuxWatcher) sendEvent(e Event) {
	w.events <- e
}

func (w *LinuxWatcher) Close() error {
	if w.isClosed {
		return nil
	}
	w.isClosed = true
	err := w.raw.Close()
	close(w.events)
	close(w.errors)
	return err
}
