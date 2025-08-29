package internal_test

import (
	"os"
	"testing"

	lib "github.com/kube-champ/terraform-runner/internal"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestTerraform(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Terraform Suite")
}

func createFile(filePath string) *os.File {
	file, _ := os.Create(filePath)

	return file
}

func writeFile(filePath string, content string) {
	file := createFile(filePath)

	file.WriteString(content)
}

func mkdir(dir string) {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		os.Mkdir(dir, 0700)
	}
}

const (
	workDir    string = "/tmp/tf-project"
	cacheDir   string = "/tmp/tf-cache"
	varDir     string = "/tmp/tf-vars"
	secretName string = "my-secret"
	kubeConfig string = "/root/.kube/config"
)

var _ = BeforeSuite(func() {
	os.Setenv("TERRAFORM_VERSION", "1.12.2")

	os.Setenv("TERRAFORM_PROJECT_PATH", workDir)
	os.Setenv("TERRAFORM_VAR_FILES_PATH", varDir)
	os.Setenv("TERRAFORM_WORKSPACE", "default")

	os.Setenv("TF_PLUGIN_CACHE_DIR", cacheDir)

	os.Setenv("OUTPUT_SECRET_NAME", secretName)
	os.Setenv("POD_NAMESPACE", "default")
	os.Setenv("KUBECONFIG", kubeConfig)

	lib.LoadEnv()

	mkdir(workDir)
	mkdir(varDir)
	mkdir(cacheDir)
})
