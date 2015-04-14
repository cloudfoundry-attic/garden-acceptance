package garden_acceptance_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestGardenAcceptance(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Garden Acceptance Suite")
}
