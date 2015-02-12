// +build darwin

package panoptes

import (
	"fmt"
	"github.com/go-fsnotify/fsevents"
	"path/filepath"
	"time"
)

type DarwinWatcher struct {
	events   chan Event
	errors   chan error
	movedTo  chan string
	raw      *fsevents.EventStream
	isClosed bool
}

func NewWatcher(path string) (w *DarwinWatcher, err error) {

	raw := &fsevents.EventStream{
		Paths:   []string{path},
		Latency: 500 * time.Millisecond,
		Flags:   fsevents.FileEvents | fsevents.NoDefer,
	}

	w = &DarwinWatcher{
		events:  make(chan Event),
		errors:  make(chan error),
		movedTo: make(chan string, 0),
		raw:     raw,
	}
	go w.translateEvents()
	w.raw.Start()
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

func (w *DarwinWatcher) translateEvents() {
	for events := range w.raw.Events {
		for _, event := range events {
			if w.raw.Paths[0] == event.Path || event.Path == filepath.Join("private", w.raw.Paths[0]) {
				if event.Flags&fsevents.ItemRemoved == fsevents.ItemRemoved {
					w.errors <- WatchedRootRemovedErr

				}
				continue
			}
			fmt.Printf("received event %s %v 0x%0X\n", event.Path, event.ID, event.Flags)
			switch {
			case event.Flags&fsevents.ItemRenamed == fsevents.ItemRenamed &&
				event.Flags&fsevents.ItemModified == fsevents.ItemModified:

				// rename started MOVED_FROM

				go func(event fsevents.Event) {
					select {
					case w.movedTo <- event.Path:
					case <-time.After(500 * time.Millisecond):
						w.sendEvent(newEvent(event.Path, Remove))
					}
				}(event)

			case event.Flags&fsevents.ItemRenamed == fsevents.ItemRenamed:
				// rename ended MOVED_TO

				go func(event fsevents.Event) {
					select {
					case oldPth := <-w.movedTo:
						w.sendEvent(newRenameEvent(event.Path, oldPth))
					case <-time.After(500 * time.Millisecond):
						w.sendEvent(newEvent(event.Path, Create))
					}
				}(event)

			case event.Flags&fsevents.ItemRemoved == fsevents.ItemRemoved:
				w.sendEvent(newEvent(event.Path, Remove))

			case event.Flags&fsevents.ItemModified == fsevents.ItemModified &&
				event.Flags&fsevents.ItemInodeMetaMod == fsevents.ItemInodeMetaMod:
				w.sendEvent(newEvent(event.Path, Modify))

			case event.Flags&fsevents.ItemCreated == fsevents.ItemCreated:
				w.sendEvent(newEvent(event.Path, Create))

			case event.Flags&fsevents.ItemIsDir == fsevents.ItemIsDir && event.Flags&fsevents.ItemCreated == fsevents.ItemCreated:
				w.sendEvent(newEvent(event.Path, Create))
			}
		}

	}
}

func (w *DarwinWatcher) sendEvent(e Event) {
	w.events <- e
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
	w.raw.Stop()
	close(w.events)
	close(w.errors)
	// close(w.raw.Events)
	return nil
}
