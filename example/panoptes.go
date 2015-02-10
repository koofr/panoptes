package main

import (
	"fmt"
	"os"

	"github.com/koofr/panoptes"
)

func main() {

	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s /path/to/watch/dir [/another/path] [/yet/another/path]\n", os.Args[0])
		return
	}

	w, err := panoptes.NewWatcher(os.Args[1:], []string{})

	if err != nil {
		fmt.Fprintf(os.Stderr, "init error: %+v\n", err)
		return
	}

	defer w.Close()

	for {
		select {
		case event := <-w.Events():
			logEvent(event)
		case err := <-w.Errors():
			fmt.Fprintf(os.Stderr, "Error %+v\n", err)
			return
		}
	}

}

var eventNames = map[panoptes.Op]string{
	panoptes.Write:  "WRITE",
	panoptes.Create: "CREATE",
	panoptes.Rename: "RENAME",
	panoptes.Remove: "REMOVE",
}

func logEvent(e panoptes.Event) {

	if e.Op == panoptes.Rename {
		fmt.Printf("RENAME: from %s to %s\n", e.OldPath, e.Path)
	} else {
		fmt.Printf("%s: %s\n", eventNames[e.Op], e.Path)
	}
}
