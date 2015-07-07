package panoptes_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/koofr/panoptes"
	"github.com/onsi/gomega"
)

func newWatcher(path string) panoptes.Watcher {
	w, err := panoptes.NewWatcher(path)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	return w
}

func closeWatcher(w panoptes.Watcher) {
	time.Sleep(250 * time.Millisecond)
	gomega.Consistently(w.Events()).ShouldNot(gomega.Receive())
	gomega.Consistently(w.Errors()).ShouldNot(gomega.Receive())
	err := w.Close()
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	gomega.Expect(w.Events()).To(gomega.BeClosed())
	gomega.Expect(w.Errors()).To(gomega.BeClosed())
}

func mkdir(path string) panoptes.Event {
	err := os.Mkdir(path, os.ModeDir|os.ModePerm)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	return panoptes.Event{
		Path:  path,
		Op:    panoptes.Create,
		IsDir: true,
	}
}

func symlink(a, b string) panoptes.Event {
	absA := a
	if !filepath.IsAbs(absA) {
		absA = filepath.Join(filepath.Dir(b), a)
	}
	info, err := os.Stat(absA)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	err = os.Symlink(a, b)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	return panoptes.Event{
		Path:  b,
		Op:    panoptes.Create,
		IsDir: info.IsDir(),
	}
}

func remove(path string) panoptes.Event {
	info, err := os.Stat(path)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	err = os.Remove(path)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	return panoptes.Event{
		Path:  path,
		Op:    panoptes.Remove,
		IsDir: info.IsDir(),
	}
}

func createFile(path string, contents string) panoptes.Event {
	err := ioutil.WriteFile(path, []byte("Hello world!"), os.ModePerm)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	return panoptes.Event{
		Path: path,
		Op:   panoptes.Create,
	}
}

func modifyFile(path string, contents string) panoptes.Event {

	fp, err := os.OpenFile(path, os.O_TRUNC|os.O_RDWR, os.ModePerm)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	var _ = fp
	_, err = fp.WriteString(contents)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	err = fp.Sync()
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	err = fp.Close()
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	return panoptes.Event{
		Path: path,
		Op:   panoptes.Modify,
	}
}

func rename(oldpth, newpth string) panoptes.Event {
	info, err := os.Stat(oldpth)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	err = os.Rename(oldpth, newpth)
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
	return panoptes.Event{
		Path:    newpth,
		OldPath: oldpth,
		Op:      panoptes.Rename,
		IsDir:   info.IsDir(),
	}
}
