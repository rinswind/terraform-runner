package internal

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	varFilesPath = "/tmp/tf-vars"
)

var _ = Describe("Helpers function", func() {
	BeforeEach(func() {
		os.RemoveAll(varFilesPath)

		dirs := []string{
			varFilesPath + "/common-config",
			varFilesPath + "/data-config",
		}

		for _, d := range dirs {
			file := d + "/data.tfvars"

			os.MkdirAll(d, os.ModePerm)
			os.WriteFile(file, []byte("length = 10"), 0644)
		}
	})

	Context("helper functions", func() {
		It("array should contain a string", func() {
			arr := []string{"abc", "deg"}

			Expect(arrayContains(arr, "abc")).To(BeTrue())
			Expect(arrayContains(arr, "123")).To(BeFalse())
		})

		It("should be a valid tfvar extension", func() {
			Expect(varFileExtensionAllowed("common.tfvars")).To(BeTrue())
			Expect(varFileExtensionAllowed("common.tf")).To(BeTrue())
			Expect(varFileExtensionAllowed("common.json")).To(BeTrue())
			Expect(varFileExtensionAllowed("common.terraform")).To(BeFalse())
		})
	})

	Context("listing files", func() {
		It("should check if file exist", func() {
			Expect(fileExists(varFilesPath + "/common-config/data.tfvars")).To(BeTrue())
		})

		It("should get all var files in nested directories", func() {
			files, err := getTfVarFilesPaths(varFilesPath)

			Expect(err).ToNot(HaveOccurred())
			Expect(files).To(HaveLen(2))

			Expect(files[0]).To(Equal(varFilesPath + "/common-config/data.tfvars"))
			Expect(files[1]).To(Equal(varFilesPath + "/data-config/data.tfvars"))
		})
	})
})
