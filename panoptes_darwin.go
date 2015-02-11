// +build darwin

package panoptes

import (
	"fmt"
	"time"

	"github.com/go-fsnotify/fsevents"
)

type DarwinWatcher struct {
	ignoredPaths []string
	events       chan Event
	errors       chan error
	renames      chan string
	renamesDone  chan error
	raw          *fsevents.EventStream
	isClosed     bool
}

func NewWatcher(paths []string, ignoredPaths []string) (w *DarwinWatcher, err error) {
	raw := &fsevents.EventStream{
		Paths:   paths,
		Latency: 500 * time.Millisecond,
		Flags:   fsevents.FileEvents,
	}

	w = &DarwinWatcher{
		ignoredPaths: ignoredPaths,
		events:       make(chan Event),
		errors:       make(chan error),
		renames:      make(chan string, 1),
		renamesDone:  make(chan error, 1),
		raw:          raw,
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

			switch {
			case event.Flags&fsevents.ItemRenamed == fsevents.ItemRenamed:
				if event.Flags&fsevents.ItemModified == fsevents.ItemModified {
					// rename started

					select {
					case w.renames <- event.Path:
						select {
						case <-w.renamesDone:
							// rename finished, all ok
						case <-time.After(750 * time.Millisecond):
							// rename did not finish
							w.sendEvent(newEvent(event.Path, Remove))
						}
					default:
						panic("FAILEDDDDDDDDD")
					}

				} else {
					select {
					case oldPth := <-w.renames:
						// rename ended
						w.sendEvent(newRenameEvent(event.Path, oldPth))
						w.sendEvent(newEvent(event.Path, Write))
						w.renamesDone <- nil
					case <-time.After(750 * time.Millisecond):
						w.sendEvent(newEvent(event.Path, Create))
						w.sendEvent(newEvent(event.Path, Write))
					}

				}

			case event.Flags&fsevents.ItemRemoved == fsevents.ItemRemoved:
				if w.isWatchedRoot(event.Path) {
					w.errors <- WatchedRootRemovedErr
				} else {
					w.sendEvent(newEvent(event.Path, Remove))
				}
			case event.Flags&fsevents.ItemModified == fsevents.ItemModified:
				w.sendEvent(newEvent(event.Path, Write))
			case event.Flags&fsevents.ItemCreated == fsevents.ItemCreated:
				w.sendEvent(newEvent(event.Path, Create))
			}
		}

	}
}

func (w *DarwinWatcher) isWatchedRoot(path string) bool {
	for _, root := range w.WatchedPaths() {
		if root == path {
			return true
		}
	}
	return false
}

func (w *DarwinWatcher) sendEvent(e Event) {
	fmt.Printf("sending %+v\n", e)
	w.events <- e
}

func (w *DarwinWatcher) Events() <-chan Event {
	return w.events
}

func (w *DarwinWatcher) Errors() <-chan error {
	return w.errors
}

func (w *DarwinWatcher) WatchedPaths() []string {
	return w.raw.Paths
}

func (w *DarwinWatcher) IgnoredPaths() []string {
	return w.ignoredPaths
}

func (w *DarwinWatcher) Close() error {
	if w.isClosed {
		return nil
	}
	w.raw.Stop()
	close(w.events)
	close(w.errors)
	close(w.raw.Events)
	return nil
}
