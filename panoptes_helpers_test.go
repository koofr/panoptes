package panoptes_test

import (
	"io/ioutil"
	"os"
	"time"

	"github.com/koofr/panoptes"
	"github.com/onsi/gomega"
)

func newWatcher(path string) panoptes.Watcher {
	w, err := panoptes.NewWatcher([]string{path}, []string{})
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	return w
}

func closeWatcher(w panoptes.Watcher) {
	time.Sleep(250 * time.Millisecond)
	gomega.Consistently(w.Events()).ShouldNot(gomega.Receive())
	gomega.Consistently(w.Errors()).ShouldNot(gomega.Receive())
	err := w.Close()
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
}

func mkdir(path string) panoptes.Event {
	err := os.Mkdir(path, os.ModeDir|os.ModePerm)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	return panoptes.Event{
		Path: path,
		Op:   panoptes.Create,
	}
}

func remove(path string) panoptes.Event {
	err := os.Remove(path)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	return panoptes.Event{
		Path: path,
		Op:   panoptes.Remove,
	}
}

func writeFile(path string, contents string) panoptes.Event {
	err := ioutil.WriteFile(path, []byte("Hello world!"), os.ModePerm)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	return panoptes.Event{
		Path: path,
		Op:   panoptes.Write,
	}
}

func rename(oldpth, newpth string) panoptes.Event {
	err := os.Rename(oldpth, newpth)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	return panoptes.Event{
		Path:    newpth,
		OldPath: oldpth,
		Op:      panoptes.Rename,
	}
}
