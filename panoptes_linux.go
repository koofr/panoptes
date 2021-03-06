// +build linux

package panoptes

import (
	"os"
	"path"
	"path/filepath"
	"strings"
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
		events:      make(chan Event, 1024),
		errors:      make(chan error),
		movedTo:     make(map[uint32]chan string),
		created:     make(map[string]chan error),
		quitCh:      make(chan error),
		raw:         watcher,
	}

	go w.translateEvents()

	if err := w.recursiveAdd(path); err != nil {
		return nil, err
	}
	return
}

func isDir(e fsnotify.Event) bool {
	return e.RawOp&syscall.IN_ISDIR == syscall.IN_ISDIR
}

func (w *LinuxWatcher) translateEvents() {
	defer func() {
		close(w.events)
		close(w.errors)
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
			case event.RawOp&syscall.IN_DELETE == syscall.IN_DELETE:
				w.events <- newEvent(event.Name, Remove, isDir(event))
			case event.RawOp&syscall.IN_DELETE_SELF == syscall.IN_DELETE_SELF:
				if w.watchedPath == event.Name {
					w.errors <- WatchedRootRemovedErr
				} else {
					continue
				}
			case event.RawOp&syscall.IN_CREATE == syscall.IN_CREATE:
				if event.RawOp&syscall.IN_ISDIR == syscall.IN_ISDIR {
					w.recursiveAdd(event.Name)
					w.events <- newEvent(event.Name, Create, isDir(event))
				} else {
					info, err := os.Stat(event.Name)
					if err != nil {
						continue
					}
					linfo, err := os.Lstat(event.Name)
					if err != nil {
						continue
					}

					if linfo.Mode()&os.ModeSymlink == os.ModeSymlink {
						if info.IsDir() {
							if lnk, err := os.Readlink(event.Name); err == nil {
								if !filepath.IsAbs(lnk) {
									lnk = filepath.Join(filepath.Dir(event.Name), lnk)
								}

								parents := []string{} // all parents of this link

								recursive := false // assume it is not recursive
								for tmp := lnk; path.Clean(tmp) != "/"; tmp = path.Dir(tmp) {
									parents = append(parents, tmp)
								}
								for _, part := range parents {
									// if any parent of link path is same file as the file link points to, it is a cycle
									statB, err := os.Stat(part)
									if err != nil {
										continue
									}
									if os.SameFile(info, statB) {
										recursive = true
										break
									}
								}

								if recursive {
									continue
								}

								if strings.HasPrefix(lnk, w.watchedPath) {
									w.recursiveAdd(event.Name)
									w.events <- newEvent(event.Name, Create, true)
								}
							}
						} else {
							w.events <- newEvent(event.Name, Create, false)
						}
					} else {
						w.createdLock.Lock()
						w.created[event.Name] = make(chan error, 1)
						w.created[event.Name] <- nil
						w.createdLock.Unlock()
					}
				}
			case event.RawOp&syscall.IN_CLOSE_WRITE == syscall.IN_CLOSE_WRITE:
				w.createdLock.RLock()
				select {
				case <-w.created[event.Name]:
					w.events <- newEvent(event.Name, Create, isDir(event))
				default:
					w.events <- newEvent(event.Name, Modify, isDir(event))
				}
				w.createdLock.RUnlock()

			case event.RawOp&syscall.IN_MOVED_FROM == syscall.IN_MOVED_FROM:
				w.movedToLock.Lock()
				w.movedTo[event.EventID] = make(chan string, 1)
				w.movedToLock.Unlock()

				go func(event fsnotify.Event) {
					w.movedToLock.RLock()
					defer w.movedToLock.RUnlock()
					select {
					case <-w.quitCh:
						return
					case newPth := <-w.movedTo[event.EventID]:
						w.events <- newRenameEvent(newPth, event.Name, isDir(event))
					case <-time.After(500 * time.Millisecond):
						w.events <- newEvent(event.Name, Remove, isDir(event))
					}
				}(event)

			case event.RawOp&syscall.IN_MOVED_TO == syscall.IN_MOVED_TO:

				w.movedToLock.RLock()
				ch := w.movedTo[event.EventID]
				w.movedToLock.RUnlock()

				go func(event fsnotify.Event) {
					select {
					case <-w.quitCh:
						return
					case ch <- event.Name:
					default:
						w.events <- newEvent(event.Name, Create, isDir(event))
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

func (w *LinuxWatcher) Close() error {
	if w.isClosed {
		return nil
	}
	w.isClosed = true
	close(w.quitCh)
	err := w.raw.Close()
	return err
}
