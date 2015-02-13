package panoptes_test

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/koofr/panoptes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Watcher", func() {

	var dir string
	var testNum int

	BeforeEach(func() {
		dir, testNum = sc.NewTest()
	})

	It("should fire event when file is created", func() {
		w := newWatcher(dir)
		defer closeWatcher(w)
		e1 := createFile(filepath.Join(dir, "file.txt"), "hello world")
		Eventually(w.Events()).Should(Receive(Equal(e1)), "receive create event")
	})

	It("should fire event when folder is created", func() {
		w := newWatcher(dir)
		defer closeWatcher(w)
		e := mkdir(filepath.Join(dir, "folder"))
		Eventually(w.Events()).Should(Receive(Equal(e)), "receive create event")
	})

	It("should fire events when file in new folder is created", func() {
		w := newWatcher(dir)
		defer closeWatcher(w)
		e1 := mkdir(filepath.Join(dir, "folder"))
		Eventually(w.Events()).Should(Receive(Equal(e1)))
		e2 := createFile(filepath.Join(dir, "folder", "file.txt"), "hello world")
		Eventually(w.Events()).Should(Receive(Equal(e2)))

	})

	It("should fire events when folder in new folder is created", func() {
		w := newWatcher(dir)
		defer closeWatcher(w)
		e1 := mkdir(filepath.Join(dir, "folder"))
		Eventually(w.Events()).Should(Receive(Equal(e1)))
		e2 := mkdir(filepath.Join(dir, "folder", "folder2"))
		Eventually(w.Events()).Should(Receive(Equal(e2)))
	})

	It("should fire events when file is deleted", func() {
		w := newWatcher(dir)
		defer closeWatcher(w)
		e1 := mkdir(filepath.Join(dir, "folder"))
		Eventually(w.Events()).Should(Receive(Equal(e1)), "receive mkdir event")
		e2 := createFile(filepath.Join(dir, "folder", "file.txt"), "hello world")
		Eventually(w.Events()).Should(Receive(Equal(e2)), "receive createFile event")
		e3 := remove(filepath.Join(dir, "folder", "file.txt"))
		Eventually(w.Events()).Should(Receive(Equal(e3)), "receive remove event")
	})

	It("should fire events when folder is deleted", func() {
		w := newWatcher(dir)
		defer closeWatcher(w)
		e1 := mkdir(filepath.Join(dir, "folder"))
		Eventually(w.Events()).Should(Receive(Equal(e1)), "receive mkdir event")
		e2 := remove(filepath.Join(dir, "folder"))
		Eventually(w.Events()).Should(Receive(Equal(e2)), "receive remove event")
	})

	It("should fire events when file is modified", func() {
		w := newWatcher(dir)
		defer closeWatcher(w)
		path := filepath.Join(dir, "test.txt")
		e1 := createFile(path, "test")
		Eventually(w.Events()).Should(Receive(Equal(e1)), "receive create file event")
		e2 := modifyFile(path, "test123")
		Eventually(w.Events()).Should(Receive(Equal(e2)), "receive modify file event")

		if runtime.GOOS == "windows" {
			Eventually(w.Events()).Should(Receive(Equal(e2)), "receive second modify file event")
		}
	})

	It("should fire event when file is renamed", func() {
		w := newWatcher(dir)
		defer closeWatcher(w)
		e1 := createFile(filepath.Join(dir, "file.txt"), "hello world")
		Eventually(w.Events()).Should(Receive(Equal(e1)))
		e2 := rename(filepath.Join(dir, "file.txt"), filepath.Join(dir, "file2.txt"))
		Eventually(w.Events(), 2*time.Second).Should(Receive(Equal(e2)))
	})

	It("should fire event when file is moved to watched folder", func() {
		oldPath := filepath.Join(dir, "..", "file.txt")
		newPath := filepath.Join(dir, "file.txt")
		createFile(oldPath, "hello world")
		w := newWatcher(dir)
		defer closeWatcher(w)
		rename(oldPath, newPath)
		Eventually(w.Events(), 3*time.Second).Should(Receive(Equal(panoptes.Event{Path: newPath, Op: panoptes.Create})))
	})

	It("should fire event when file is moved out of watched folder", func() {
		oldPath := filepath.Join(dir, "file.txt")
		newPath := filepath.Join(dir, "..", "file.txt")
		createFile(oldPath, "hello world")
		w := newWatcher(dir)
		defer closeWatcher(w)
		rename(oldPath, newPath)
		Eventually(w.Events(), 3*time.Second).Should(Receive(Equal(panoptes.Event{Path: oldPath, Op: panoptes.Remove})))
	})

	It("should report error when watched folder is removed", func() {
		w := newWatcher(dir)
		defer closeWatcher(w)
		os.Remove(dir)
		Eventually(w.Errors(), 3*time.Second).Should(Receive(Equal(panoptes.WatchedRootRemovedErr)))
	})

	It("should quit properly", func() {
		w := newWatcher(dir)
		w.Close()
		createFile(filepath.Join(dir, "annoy.me"), "hello world")
		Eventually(w.Errors()).Should(BeClosed())
		Eventually(w.Events()).Should(BeClosed())
	})

	Context("with a lot of files", func() {

		n := 100

		It("should work when hundreds of files are created at once", func() {
			w := newWatcher(dir)
			defer closeWatcher(w)

			for i := 0; i < n; i++ {
				e := createFile(filepath.Join(dir, fmt.Sprintf("file%d.txt", i)), "ohai")
				Eventually(w.Events(), 2*time.Second).Should(Receive(Equal(e)))
			}
		})

		It("should work when hundreds of files are deleted at once", func() {

			for i := 0; i < n; i++ {
				createFile(filepath.Join(dir, fmt.Sprintf("file%d.txt", i)), "ohai")
			}

			time.Sleep(3 * time.Second)

			w := newWatcher(dir)
			defer closeWatcher(w)

			for i := 0; i < n; i++ {
				e := remove(filepath.Join(dir, fmt.Sprintf("file%d.txt", i)))
				Eventually(w.Events(), 2*time.Second).Should(Receive(Equal(e)))
			}
		})

		It("should work when hundreds of files are renamed at once", func() {

			for i := 0; i < n; i++ {
				createFile(filepath.Join(dir, fmt.Sprintf("file%d.txt", i)), "ohai")
			}
			time.Sleep(3 * time.Second)

			w := newWatcher(dir)
			defer closeWatcher(w)

			for i := 0; i < n; i++ {
				oldPth := filepath.Join(dir, fmt.Sprintf("file%d.txt", i))
				newPth := filepath.Join(dir, fmt.Sprintf("a_file%d.txt", i))
				e := rename(oldPth, newPth)
				Eventually(w.Events(), 4*time.Second).Should(Receive(Equal(e)))
			}
		})

		It("should work when hundreds of files are modified at once", func() {

			for i := 0; i < n; i++ {
				createFile(filepath.Join(dir, fmt.Sprintf("file%d.txt", i)), "ohai")
			}
			w := newWatcher(dir)
			defer closeWatcher(w)

			for i := 0; i < n; i++ {
				e := modifyFile(filepath.Join(dir, fmt.Sprintf("file%d.txt", i)), "hello world")
				Eventually(w.Events()).Should(Receive(Equal(e)), "receive modify event")
				if runtime.GOOS == "windows" {
					Eventually(w.Events(), 2*time.Second).Should(Receive(Equal(e)), "receive second modify file event")
				}
			}
		})

		It("should work when hundreds of folders are created at once", func() {
			w := newWatcher(dir)
			defer closeWatcher(w)

			for i := 0; i < n; i++ {
				e := mkdir(filepath.Join(dir, fmt.Sprintf("folder%d", i)))
				Eventually(w.Events()).Should(Receive(Equal(e)))
			}
		})

		It("should work when hundreds of folders are deleted at once", func() {

			for i := 0; i < n; i++ {
				mkdir(filepath.Join(dir, fmt.Sprintf("folder%d", i)))
			}
			w := newWatcher(dir)
			defer closeWatcher(w)

			for i := 0; i < n; i++ {
				e := remove(filepath.Join(dir, fmt.Sprintf("folder%d", i)))
				Eventually(w.Events()).Should(Receive(Equal(e)))
			}
		})

		It("should work when hundreds of folders are renamed at once", func() {

			for i := 0; i < n; i++ {
				mkdir(filepath.Join(dir, fmt.Sprintf("folder%d", i)))
			}
			w := newWatcher(dir)
			defer closeWatcher(w)

			for i := 0; i < n; i++ {
				oldPth := filepath.Join(dir, fmt.Sprintf("folder%d", i))
				newPth := filepath.Join(dir, fmt.Sprintf("a_folder%d", i))
				e := rename(oldPth, newPth)
				Eventually(w.Events(), 4*time.Second).Should(Receive(Equal(e)))
			}
		})
	})
})
