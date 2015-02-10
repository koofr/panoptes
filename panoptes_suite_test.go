package panoptes_test

import (
	"github.com/koofr/ginkgoutils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

var (
	sc = ginkgoutils.NewSuiteConfig("github.com/koofr/panoptes")
)

func TestWatcher(t *testing.T) {
	RegisterFailHandler(sc.Fail)
	RunSpecs(t, "Panoptes Suite")
}

var _ = BeforeSuite(sc.SetupSuite)
var _ = AfterSuite(sc.CleanupSuite)
