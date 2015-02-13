// +build darwin

package panoptes

import (
	"fmt"
	"github.com/go-fsnotify/fsevents"
	"os"
	"path/filepath"
	"time"
)

var _ = fmt.Println

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
		events:  make(chan Event, 256),
		errors:  make(chan error),
		movedTo: make(chan string, 256),
		raw:     raw,
	}
	go w.processRenames()
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
	defer close(w.events)
	defer close(w.errors)
	defer close(w.movedTo)
	for events := range w.raw.Events {
		for _, event := range events {
			if w.isClosed {
				return
			}
			if w.raw.Paths[0] == event.Path || event.Path == filepath.Join("private", w.raw.Paths[0]) {
				if event.Flags&fsevents.ItemRemoved == fsevents.ItemRemoved {
					w.errors <- WatchedRootRemovedErr

				}
				continue
			}
			fmt.Printf("%s got raw event %s 0x%0X\n", time.Now(), event.Path, event.Flags)
			switch {
			case event.Flags&fsevents.ItemRenamed == fsevents.ItemRenamed:
				w.movedTo <- event.Path
			case event.Flags&fsevents.ItemRemoved == fsevents.ItemRemoved:
				go w.sendEvent(newEvent(event.Path, Remove))

			case event.Flags&fsevents.ItemModified == fsevents.ItemModified &&
				event.Flags&fsevents.ItemInodeMetaMod == fsevents.ItemInodeMetaMod:
				go w.sendEvent(newEvent(event.Path, Modify))

			case event.Flags&fsevents.ItemCreated == fsevents.ItemCreated:
				go w.sendEvent(newEvent(event.Path, Create))

			}
		}

	}
}

func (w *DarwinWatcher) processRenames() {
	oldPath := ""
	for {
		if w.isClosed {
			return
		}
		select {
		case path := <-w.movedTo:
			if oldPath != "" {
				go w.sendEvent(newRenameEvent(path, oldPath))
				oldPath = ""
			} else {
				oldPath = path
			}
		case <-time.After(1 * time.Second):
			if oldPath != "" {
				_, err := os.Stat(oldPath)
				if os.IsNotExist(err) {
					go w.sendEvent(newEvent(oldPath, Remove))
				}
				if err == nil {
					go w.sendEvent(newEvent(oldPath, Create))
				}
				oldPath = ""
			}
		}
	}

}

func (w *DarwinWatcher) sendEvent(e Event) {
	if !w.isClosed {
		w.events <- e
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
	w.raw.Stop()
	// close(w.raw.Events)
	return nil
}
