package hypershift_test

import (
	"os/exec"

	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/stolostron/hypershift-addon-e2e-tests/e2e-go/pkg/utils"
)

var _ = ginkgo.Describe("Hypershift Add-on Must-Gather Tests:", ginkgo.Label("@must-gather"), func() {

	ginkgo.It("Triggers must-gather on a particular hosted cluster", func() {
		// TODO: get a hosted cluster and its namespace if HCP is provided
		// TOOD: pick random hosted cluster if not provided or not found
		// TODO: run must-gather on the hub using hosted cluster
		// TODO: param needed: must-gather image -> if not provided, test will skip.

		// assumes running from /e2e-go/pkg/test
		pathToScript := "./../../../scripts/must-gather/run_must_gather_hcp.sh"

		cmd := exec.Command("/bin/sh", pathToScript)

		session, err := gexec.Start(cmd, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
		defer gexec.KillAndWait()
		utils.PrintOutput(session) // prints command, args and output
		gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		gomega.Eventually(session, eventuallyTimeout, eventuallyInterval).Should(gexec.Exit(0))
	})
})
