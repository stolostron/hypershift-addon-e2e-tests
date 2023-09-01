package hypershift_test

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"

	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
	"github.com/stolostron/hypershift-addon-e2e-tests/e2e-go/pkg/utils"
	"k8s.io/apimachinery/pkg/api/errors"
)

const (
	HCP_CONSOLE_DISPL_NAME = "hcp - Hosted Control Plane Command Line Interface (CLI)"
	HCP_CONSOLE_DESC       = "With the Hosted Control Plane command line interface, you can create and manage OpenShift hosted clusters.\n"
	HCP_FILE_NAME          = "hcp.tar.gz"
	LINUX_AMD64            = "linux/amd64"
	LINUX_AMD64_DESC       = "Download hcp CLI for Linux for x86_64"
	LINUX_ARM64            = "linux/arm64"
	LINUX_ARM64_DESC       = "Download hcp CLI for Linux for ARM 64"
	MAC_AMD64              = "darwin/amd64"
	MAC_AMD64_DESC         = "Download hcp CLI for Mac for x86_64"
	MAC_ARM64              = "darwin/arm64"
	MAC_ARM64_DESC         = "Download hcp CLI for Mac for ARM 64"
	WINDOWS_AMD64          = "windows/amd64"
	WINDOWS_AMD64_DESC     = "Download hcp CLI for Windows for x86_64"
	WINDOWS_ARM64          = "windows/arm64"
	WINDOWS_ARM64_DESC     = "Download hcp CLI for Windows for ARM 64"
)

func checkConsoleCLIDownloadLink(links []interface{}, osArch string) error {
	for _, link := range links {
		if linkMap, isMap := link.(map[string]interface{}); isMap {
			if strings.Contains(linkMap["href"].(string), osArch) {
				fmt.Printf("linkMap[href]: %+v\n", linkMap["href"])
				fmt.Printf("linkMap[text]: %+v\n", linkMap["text"])

				// now use the href to test the url
				tr := &http.Transport{
					TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
				}
				client := &http.Client{Transport: tr}

				res, err := client.Get(linkMap["href"].(string))
				if err != nil {
					return err
				}
				defer res.Body.Close()
				gomega.Expect(res.StatusCode).To(gomega.Equal(200))

				break
			}
		}
	}
	return nil
}

var _ = ginkgo.Describe("Hosted Control Plane CLI Binary Tests", ginkgo.Label("e2e", "create", TYPE_AWS), func() {
	// TODO: krew-manager works as expected?

	// TODO
	// ginkgo.Context("irrespective of whether the console is enabled or not", func() {
	// 	// TODO: route is good for each OS/arch
	// 	ginkgo.BeforeEach(func() {
	// 		// skip if route is nil
	// 	})

	// 	ginkgo.It("should have working urls for each os/arch", ginkgo.Label("e2e", "label", "console"), func() {
	// 	})
	// })

	ginkgo.Context("the console is enabled", func() {

		ginkgo.BeforeEach(func() {
			if hcpCliConsoleDownloadSpec == nil {
				ginkgo.Skip("hcpCliConsoleDownloadSpec is not set, perhaps console is not enabled?")
			}
		})

		// TODO
		ginkgo.It("should no longer have the oid hypershift console link refernce", ginkgo.Label("e2e", "label", "console"), func() {
			ginkgo.Skip("WIP")
			_, err = utils.GetConsoleCliDownload(dynamicClient, "hypershift-cli-download")
			// expect err to be errors.IsNotFound
			gomega.Expect(err).To(gomega.HaveOccurred())
			gomega.Expect(err).Should(gomega.MatchError(errors.IsNotFound))
		})

		ginkgo.It("should have the correct ConsoleCLIDownload display name set for hcp", ginkgo.Label("e2e", "label", "console"), func() {
			fmt.Printf("hcpCLIDownload display name: %+v\n", hcpCliConsoleDownloadSpec["displayName"])
			gomega.Expect(hcpCliConsoleDownloadSpec["displayName"]).To(gomega.Equal(HCP_CONSOLE_DISPL_NAME))
		})

		ginkgo.It("should have the correct ConsoleCLIDownload description set for hcp", ginkgo.Label("e2e", "label", "console"), func() {
			fmt.Printf("hcpCLIDownload description: %+v\n", hcpCliConsoleDownloadSpec["description"])
			gomega.Expect(hcpCliConsoleDownloadSpec["description"]).To(gomega.Equal(HCP_CONSOLE_DESC))
		})

		ginkgo.When("a user is downloading the hcp binary from MCE/ACM", func() {

			ginkgo.It("should have the correct link for Linux x86_64", ginkgo.Label("e2e", "label", "consoleLinks"), func() {
				arch := LINUX_AMD64
				ginkgo.By(fmt.Sprintf("Verifying if href and description for os/arch %s exists and is correct\n", arch))
				gomega.Expect(hcpCliConsoleDownloadSpec["links"]).To(gomega.ContainElement(gstruct.MatchKeys(gstruct.IgnoreExtras, gstruct.Keys{
					"href": gomega.ContainSubstring(arch + "/" + HCP_FILE_NAME),
					"text": gomega.ContainSubstring(LINUX_AMD64_DESC),
				})))

				ginkgo.By(fmt.Sprintf("Verifying HTTP GET on file href for os/arch %s works as expected \n", arch))
				if links, ok := hcpCliConsoleDownloadSpec["links"].([]interface{}); ok {
					checkConsoleCLIDownloadLink(links, arch)
				}
			})

			ginkgo.It("should have the correct link for Linux ARM 64", ginkgo.Label("e2e", "label", "consoleLinks"), func() {

				arch := LINUX_ARM64
				ginkgo.By(fmt.Sprintf("Verifying if href and description for os/arch %s exists and is correct\n", arch))
				gomega.Expect(hcpCliConsoleDownloadSpec["links"]).To(gomega.ContainElement(gstruct.MatchKeys(gstruct.IgnoreExtras, gstruct.Keys{
					"href": gomega.ContainSubstring(arch + "/" + HCP_FILE_NAME),
					"text": gomega.ContainSubstring(LINUX_ARM64_DESC),
				})))

				ginkgo.By(fmt.Sprintf("Verifying HTTP GET on file href for os/arch %s works as expected \n", arch))
				if links, ok := hcpCliConsoleDownloadSpec["links"].([]interface{}); ok {
					checkConsoleCLIDownloadLink(links, arch)
				}
			})

			ginkgo.It("should have the correct link for Mac x86_64", ginkgo.Label("e2e", "label", "consoleLinks"), func() {

				arch := MAC_AMD64
				ginkgo.By(fmt.Sprintf("Verifying if href and description for os/arch %s exists and is correct\n", arch))
				gomega.Expect(hcpCliConsoleDownloadSpec["links"]).To(gomega.ContainElement(gstruct.MatchKeys(gstruct.IgnoreExtras, gstruct.Keys{
					"href": gomega.ContainSubstring(arch + "/" + HCP_FILE_NAME),
					"text": gomega.ContainSubstring(MAC_AMD64_DESC),
				})))

				ginkgo.By(fmt.Sprintf("Verifying HTTP GET on file href for os/arch %s works as expected \n", arch))
				if links, ok := hcpCliConsoleDownloadSpec["links"].([]interface{}); ok {
					checkConsoleCLIDownloadLink(links, arch)
				}
			})

			ginkgo.It("should have the correct link for Mac ARM 64", ginkgo.Label("e2e", "label", "consoleLinks"), func() {

				arch := MAC_ARM64
				ginkgo.By(fmt.Sprintf("Verifying if href and description for os/arch %s exists and is correct\n", arch))
				gomega.Expect(hcpCliConsoleDownloadSpec["links"]).To(gomega.ContainElement(gstruct.MatchKeys(gstruct.IgnoreExtras, gstruct.Keys{
					"href": gomega.ContainSubstring(arch + "/" + HCP_FILE_NAME),
					"text": gomega.ContainSubstring(MAC_ARM64_DESC),
				})))

				ginkgo.By(fmt.Sprintf("Verifying HTTP GET on file href for os/arch %s works as expected \n", arch))
				if links, ok := hcpCliConsoleDownloadSpec["links"].([]interface{}); ok {
					checkConsoleCLIDownloadLink(links, arch)
				}
			})

			ginkgo.It("should have the correct link for Windows x86_64", ginkgo.Label("e2e", "label", "consoleLinks"), func() {

				arch := WINDOWS_AMD64
				ginkgo.By(fmt.Sprintf("Verifying if href and description for os/arch %s exists and is correct\n", arch))
				gomega.Expect(hcpCliConsoleDownloadSpec["links"]).To(gomega.ContainElement(gstruct.MatchKeys(gstruct.IgnoreExtras, gstruct.Keys{
					"href": gomega.ContainSubstring(arch + "/" + HCP_FILE_NAME),
					"text": gomega.ContainSubstring(WINDOWS_AMD64_DESC),
				})))

				ginkgo.By(fmt.Sprintf("Verifying HTTP GET on file href for os/arch %s works as expected \n", arch))
				if links, ok := hcpCliConsoleDownloadSpec["links"].([]interface{}); ok {
					checkConsoleCLIDownloadLink(links, arch)
				}
			})
		})

	})
})
