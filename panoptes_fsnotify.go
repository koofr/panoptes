// +build !darwin

package panoptes

import (
	"path/filepath"
	"sync"

	"github.com/koofr/fsnotify"
)

type LinWinWatcher struct {
	watchedPaths []string
	ignoredPaths []string
	events       chan Event
	errors       chan error
	renames      map[uint32]string
	renamesLock  sync.Mutex
	raw          *fsnotify.Watcher
	isClosed     bool
}

func (w *LinWinWatcher) Events() <-chan Event {
	return w.events
}

func (w *LinWinWatcher) Errors() <-chan error {
	return w.errors
}

func (w *LinWinWatcher) WatchedPaths() []string {
	return w.watchedPaths
}

func (w *LinWinWatcher) IgnoredPaths() []string {
	return w.ignoredPaths
}

func (w *LinWinWatcher) translateErrors() {
	for err := range w.raw.Errors {
		w.errors <- err
	}
}

func (w *LinWinWatcher) isWatchedRoot(path string) bool {
	for _, root := range w.watchedPaths {
		if root == path {
			return true
		}
	}
	return false
}

func (w *LinWinWatcher) isIgnoredPath(pth string) bool {
	for _, ignore := range w.ignoredPaths {
		if ok, err := filepath.Match(ignore, pth); ok && err == nil {
			return true
		}
	}
	return false
}

func (w *LinWinWatcher) sendEvent(e Event) {
	w.events <- e
}

func (w *LinWinWatcher) Close() error {
	if w.isClosed {
		return nil
	}
	w.isClosed = true
	err := w.raw.Close()
	close(w.events)
	close(w.errors)
	return err
}
