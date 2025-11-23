package tinyhiera

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestTinyhiera(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "TinyHiera Resolver Suite")
}
