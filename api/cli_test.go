package api_test

import (
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("CLI Downloads API", func() {
	var (
		response *http.Response
	)

	BeforeEach(func() {
		var err error

		darwinDurr := filepath.Join(cliDownloadsDir, "darwin", "amd64")
		windowsDurr := filepath.Join(cliDownloadsDir, "windows", "amd64")

		err = os.MkdirAll(darwinDurr, 0755)
		Expect(err).NotTo(HaveOccurred())

		err = os.MkdirAll(windowsDurr, 0755)
		Expect(err).NotTo(HaveOccurred())

		err = ioutil.WriteFile(filepath.Join(darwinDurr, "fly"), []byte("soi soi soi"), 0644)
		Expect(err).NotTo(HaveOccurred())

		err = ioutil.WriteFile(filepath.Join(windowsDurr, "fly"), []byte("soi soi soi.notavirus.bat"), 0644)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(cliDownloadsDir)
	})

	Describe("GET /api/v1/cli?platform=darwin&arch=amd64", func() {
		JustBeforeEach(func() {
			req, err := http.NewRequest("GET", server.URL+"/api/v1/cli?platform=darwin&arch=amd64", nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns 200", func() {
			Expect(response.StatusCode).To(Equal(http.StatusOK))
		})

		It("sets the filename as 'fly'", func() {
			Expect(response.Header.Get("Content-Disposition")).To(Equal("attachment; filename=fly"))
		})

		It("returns the file binary", func() {
			Expect(ioutil.ReadAll(response.Body)).To(Equal([]byte("soi soi soi")))
		})
	})

	Describe("GET /api/v1/cli?platform=windows&arch=amd64", func() {
		JustBeforeEach(func() {
			req, err := http.NewRequest("GET", server.URL+"/api/v1/cli?platform=windows&arch=amd64", nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns 200", func() {
			Expect(response.StatusCode).To(Equal(http.StatusOK))
		})

		It("sets the filename as 'fly.exe'", func() {
			Expect(response.Header.Get("Content-Disposition")).To(Equal("attachment; filename=fly.exe"))
		})

		It("returns the file binary", func() {
			Expect(ioutil.ReadAll(response.Body)).To(Equal([]byte("soi soi soi.notavirus.bat")))
		})
	})

	Describe("GET /api/v1/cli?platform=darwin&arch=../darwin/amd64", func() {
		JustBeforeEach(func() {
			req, err := http.NewRequest("GET", server.URL+"/api/v1/cli?platform=darwin&arch=../darwin/amd64", nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns Bad Request", func() {
			Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
		})
	})

	Describe("GET /api/v1/cli?platform=../etc/passwd&arch=amd64", func() {
		JustBeforeEach(func() {
			req, err := http.NewRequest("GET", server.URL+"/api/v1/cli?platform=../etc/passwd&arch=amd64", nil)
			Expect(err).NotTo(HaveOccurred())

			response, err = client.Do(req)
			Expect(err).NotTo(HaveOccurred())
		})

		It("returns Bad Request", func() {
			Expect(response.StatusCode).To(Equal(http.StatusBadRequest))
		})
	})
})
