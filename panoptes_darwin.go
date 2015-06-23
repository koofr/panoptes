// +build darwin

package panoptes

import (
	"path/filepath"
	"time"

	"github.com/koofr/fsevents"
)

type DarwinWatcher struct {
	events   chan Event
	errors   chan error
	raw      *fsevents.EventStream
	isClosed bool
	quitCh   chan error
}

func NewWatcher(path string) (w *DarwinWatcher, err error) {

	raw := &fsevents.EventStream{
		Paths:   []string{path},
		Latency: 1 * time.Millisecond,
		Flags:   fsevents.FileEvents | fsevents.NoDefer,
	}

	w = &DarwinWatcher{
		events: make(chan Event, 128),
		errors: make(chan error),
		quitCh: make(chan error),
		raw:    raw,
	}
	w.raw.Start()
	go w.translateEvents()

	return
}

var noteDescription = map[fsevents.EventFlags]string{
	fsevents.MustScanSubDirs: "MustScanSubdirs",
	fsevents.UserDropped:     "UserDropped",
	fsevents.KernelDropped:   "KernelDropped",
	fsevents.EventIDsWrapped: "EventIDsWrapped",
	fsevents.HistoryDone:     "HistoryDone",
	fsevents.RootChanged:     "RootChanged",
	fsevents.Mount:           "Mount",
	fsevents.Unmount:         "Unmount",

	fsevents.ItemCreated:       "Created",
	fsevents.ItemRemoved:       "Removed",
	fsevents.ItemInodeMetaMod:  "InodeMetaMod",
	fsevents.ItemRenamed:       "Renamed",
	fsevents.ItemModified:      "Modified",
	fsevents.ItemFinderInfoMod: "FinderInfoMod",
	fsevents.ItemChangeOwner:   "ChangeOwner",
	fsevents.ItemXattrMod:      "XAttrMod",
	fsevents.ItemIsFile:        "IsFile",
	fsevents.ItemIsDir:         "IsDir",
	fsevents.ItemIsSymlink:     "IsSymLink",
}

func isDir(e fsevents.Event) bool {
	return e.Flags&fsevents.ItemIsDir == fsevents.ItemIsDir
}

func (w *DarwinWatcher) translateEvents() {

	defer func() {
		close(w.events)
		close(w.errors)
	}()

	for {
		select {
		case <-w.quitCh:
			return
		case events, ok := <-w.raw.Events:
			if !ok {
				return
			}

			for _, event := range events {
				if w.raw.Paths[0] == event.Path || event.Path == filepath.Join("private", w.raw.Paths[0]) {
					if event.Flags&fsevents.ItemRemoved == fsevents.ItemRemoved {
						w.errors <- WatchedRootRemovedErr
					}
					continue
				}
				switch {
				case event.Flags&fsevents.ItemRenamed == fsevents.ItemRenamed:
					w.events <- newEvent(event.Path, Rename, isDir(event))
				case event.Flags&fsevents.ItemRemoved == fsevents.ItemRemoved:
					w.events <- newEvent(event.Path, Remove, isDir(event))
				case event.Flags&fsevents.ItemModified == fsevents.ItemModified &&
					event.Flags&fsevents.ItemInodeMetaMod == fsevents.ItemInodeMetaMod:
					w.events <- newEvent(event.Path, Modify, isDir(event))
				case event.Flags&fsevents.ItemCreated == fsevents.ItemCreated:
					w.events <- newEvent(event.Path, Create, isDir(event))
				}
			}
		}
	}
}

func (w *DarwinWatcher) Events() <-chan Event {
	return w.events
}

func (w *DarwinWatcher) Errors() <-chan error {
	return w.errors
}

func (w *DarwinWatcher) Close() error {
	if w.isClosed {
		return nil
	}
	w.isClosed = true
	close(w.quitCh)
	w.raw.Stop()
	return nil
}
