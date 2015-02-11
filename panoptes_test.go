package panoptes_test

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/koofr/panoptes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Watcher", func() {

	var w panoptes.Watcher
	var dir string
	var testNum int

	BeforeEach(func() {
		dir, testNum = sc.NewTest()
	})

	It("should fire event when file is created", func() {
		w = newWatcher(dir)
		defer closeWatcher(w)
		e1 := writeFile(filepath.Join(dir, "file.txt"), "hello world")
		Eventually(w.Events()).Should(Receive(Equal(e1)))
	})

	It("should fire event when folder is created", func() {
		w = newWatcher(dir)
		defer closeWatcher(w)
		e := mkdir(filepath.Join(dir, "folder"))
		Eventually(w.Events()).Should(Receive(Equal(e)))
	})

	It("should fire events when file in new folder is created", func() {
		w = newWatcher(dir)
		defer closeWatcher(w)
		e1 := mkdir(filepath.Join(dir, "folder"))
		Eventually(w.Events()).Should(Receive(Equal(e1)))
		e3 := writeFile(filepath.Join(dir, "folder", "file.txt"), "hello world")
		Eventually(w.Events()).Should(Receive(Equal(e3)))

	})

	It("should fire events when file in new folder is created", func() {
		w = newWatcher(dir)
		defer closeWatcher(w)
		e1 := mkdir(filepath.Join(dir, "folder"))
		Eventually(w.Events()).Should(Receive(Equal(e1)))
		e3 := writeFile(filepath.Join(dir, "folder", "file.txt"), "hello world")
		Eventually(w.Events()).Should(Receive(Equal(e3)))
	})

	It("should fire events when file is deleted", func() {
		w = newWatcher(dir)
		defer closeWatcher(w)
		e1 := mkdir(filepath.Join(dir, "folder"))
		Eventually(w.Events()).Should(Receive(Equal(e1)), "receive mkdir event")
		e3 := writeFile(filepath.Join(dir, "folder", "file.txt"), "hello world")
		Eventually(w.Events()).Should(Receive(Equal(e3)), "receive writeFile event")
		e4 := remove(filepath.Join(dir, "folder", "file.txt"))
		Eventually(w.Events()).Should(Receive(Equal(e4)), "receive remove event")
	})

	It("should fire events when folder is deleted", func() {
		w = newWatcher(dir)
		defer closeWatcher(w)
		e1 := mkdir(filepath.Join(dir, "folder"))
		Eventually(w.Events()).Should(Receive(Equal(e1)), "receive mkdir event")
		e3 := remove(filepath.Join(dir, "folder"))
		Eventually(w.Events()).Should(Receive(Equal(e3)), "receive remove event")
	})

	It("should fire events when file is modified", func() {
		w = newWatcher(dir)
		defer closeWatcher(w)
		path := filepath.Join(dir, "test.txt")
		e2 := writeFile(path, "test")
		Eventually(w.Events()).Should(Receive(Equal(e2)), "receive write file event")

		fp, err := os.OpenFile(path, os.O_APPEND|os.O_RDWR, os.ModePerm)
		Expect(err).NotTo(HaveOccurred())

		_, err = fp.WriteString("something new")
		Expect(err).NotTo(HaveOccurred())

		fp.Close()
		Eventually(w.Events()).Should(Receive(Equal(panoptes.Event{Path: path, Op: panoptes.Write})))
	})

	It("should fire event when file is renamed", func() {
		w = newWatcher(dir)
		defer closeWatcher(w)
		e1 := writeFile(filepath.Join(dir, "file.txt"), "hello world")
		Eventually(w.Events()).Should(Receive(Equal(e1)))

		e2 := rename(filepath.Join(dir, "file.txt"), filepath.Join(dir, "file2.txt"))
		Eventually(w.Events(), 3*time.Second).Should(Receive(Equal(e2)))
		Eventually(w.Events(), 3*time.Second).Should(Receive(Equal(panoptes.Event{Path: filepath.Join(dir, "file2.txt"), Op: panoptes.Write})))

	})

	It("should fire event when file is moved to watched folder", func() {
		w = newWatcher(dir)
		defer closeWatcher(w)
		oldPath := filepath.Join(dir, "..", "file.txt")
		newPath := filepath.Join(dir, "file.txt")
		writeFile(oldPath, "hello world")
		rename(oldPath, newPath)
		Eventually(w.Events(), 3*time.Second).Should(Receive(Equal(panoptes.Event{Path: newPath, Op: panoptes.Create})))
		Eventually(w.Events(), 3*time.Second).Should(Receive(Equal(panoptes.Event{Path: newPath, Op: panoptes.Write})))
	})

	It("should fire event when file is moved out of watched folder", func() {
		w = newWatcher(dir)
		defer closeWatcher(w)
		oldPath := filepath.Join(dir, "file.txt")
		newPath := filepath.Join(dir, "..", "file.txt")
		writeFile(oldPath, "hello world")
		rename(oldPath, newPath)
		Eventually(w.Events(), 3*time.Second).Should(Receive(Equal(panoptes.Event{Path: oldPath, Op: panoptes.Remove})))
	})

	It("should report error when watched folder is removed", func() {
		w = newWatcher(dir)
		defer closeWatcher(w)
		os.Remove(dir)
		Eventually(w.Errors()).Should(Receive(Equal(panoptes.WatchedRootRemovedErr)))
	})

	It("should work when hundreds of files are created at once", func() {
		w = newWatcher(dir)
		defer closeWatcher(w)

		n := 500

		for i := 0; i < n; i++ {
			e := writeFile(filepath.Join(dir, fmt.Sprintf("file%d.txt", i)), "ohai")
			Eventually(w.Events()).Should(Receive(Equal(e)))
		}
	})

	It("should work when hundreds of files are deleted at once", func() {

		n := 500
		for i := 0; i < n; i++ {
			writeFile(filepath.Join(dir, fmt.Sprintf("file%d.txt", i)), "ohai")
		}
		w = newWatcher(dir)
		defer closeWatcher(w)

		for i := 0; i < n; i++ {
			e := remove(filepath.Join(dir, fmt.Sprintf("file%d.txt", i)))
			Eventually(w.Events()).Should(Receive(Equal(e)))
		}
	})

	It("should work when hundreds of files are renamed at once", func() {
		n := 500
		for i := 0; i < n; i++ {
			writeFile(filepath.Join(dir, fmt.Sprintf("file%d.txt", i)), "ohai")
		}
		w = newWatcher(dir)
		defer closeWatcher(w)

		for i := 0; i < n; i++ {
			oldPth := filepath.Join(dir, fmt.Sprintf("file%d.txt", i))
			newPth := filepath.Join(dir, fmt.Sprintf("a_file%d.txt", i))
			e2 := rename(oldPth, newPth)
			Eventually(w.Events()).Should(Receive(Equal(e2)))
			Eventually(w.Events()).Should(Receive(Equal(panoptes.Event{Path: newPth, Op: panoptes.Write})))
		}
	})

})
