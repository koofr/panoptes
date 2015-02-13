package panoptes_test

import (
	"github.com/koofr/ginkgoutils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"runtime"
	"testing"
	"time"
)

var (
	sc = ginkgoutils.NewSuiteConfig("github.com/koofr/panoptes")
)

func TestWatcher(t *testing.T) {
	RegisterFailHandler(sc.Fail)
	RunSpecs(t, "Panoptes Suite")

	if runtime.GOOS == "darwin" {
		SetDefaultEventuallyTimeout(5 * time.Second)
	} else {
		SetDefaultEventuallyTimeout(2 * time.Second)

	}

}

var _ = BeforeSuite(sc.SetupSuite)
var _ = AfterSuite(sc.CleanupSuite)
