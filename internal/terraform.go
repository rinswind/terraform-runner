package internal

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
	"github.com/hashicorp/go-version"
	"github.com/hashicorp/hc-install/product"
	"github.com/hashicorp/hc-install/releases"
	"github.com/hashicorp/terraform-exec/tfexec"

	log "github.com/sirupsen/logrus"
)

const (
	terraformCacheLockFile    = ".terraform-init.lock"
	terraformCacheLockTimeout = 5 * time.Minute
	terraformCacheLockPoll    = 500 * time.Millisecond
)

type TerraformRunner struct {
	TerraformVersion string
	ProjectDir       string
	CacheDir         string
	VarFilesPath     string

	varFiles  []string
	cmd       *tfexec.Terraform
	cacheLock *flock.Flock
	log       *log.Entry
}

func NewTerraformRunner(version, projectDir, cacheDir string, varFilePath string) *TerraformRunner {
	return &TerraformRunner{
		TerraformVersion: version,
		ProjectDir:       projectDir,
		CacheDir:         cacheDir,
		VarFilesPath:     varFilePath,
	}
}

func (tr *TerraformRunner) Setup() error {
	tr.log = log.
		WithField("version", tr.TerraformVersion).
		WithField("projectDir", tr.ProjectDir).
		WithField("cacheDir", tr.CacheDir)

	lockPath := filepath.Join(tr.CacheDir, terraformCacheLockFile)
	tr.cacheLock = flock.New(lockPath)

	// Install terraform binary
	execPath, err := tr.install()
	if err != nil {
		tr.log.WithField("error", err).Error("failed to install Terraform")
		return err
	}

	// Setup a terraform exec
	cmd, err := tfexec.NewTerraform(tr.ProjectDir, execPath)
	if err != nil {
		tr.log.WithField("error", err).Error("failed running NewTerraform")
		return err
	}

	cmd.SetLogger(tr.log)

	tr.cmd = cmd

	// Find the var files
	varFiles, err := getTfVarFilesPaths(tr.VarFilesPath)
	if err != nil {
		tr.log.WithField("error", err).Error("failed to list files in the var files path")
		return err
	}

	tr.varFiles = varFiles

	return nil
}

func (tr *TerraformRunner) install() (string, error) {
	err := tr.obtainCacheDirLock()
	if err != nil {
		return "", err
	}
	defer tr.releaseCacheLock()

	cachedExecPath := filepath.Join(tr.CacheDir, fmt.Sprintf("terraform-%s", tr.TerraformVersion))

	// Check whether there's a file at cachedExecPath
	if _, err := os.Stat(cachedExecPath); err == nil {
		tr.log.WithField("path", cachedExecPath).Info("found cached terraform binary")
		return cachedExecPath, nil
	}

	// Install terraform to the working_dir
	tr.log.Info("installing terraform")

	installer := &releases.ExactVersion{
		Product:    product.Terraform,
		Version:    version.Must(version.NewVersion(tr.TerraformVersion)),
		InstallDir: tr.CacheDir,
	}

	execPath, err := installer.Install(context.Background())
	if err != nil {
		return "", err
	}

	if err := os.Rename(execPath, cachedExecPath); err != nil {
		return "", fmt.Errorf("failed to move terraform binary to cache dir: %w", err)
	}

	if err := os.Chmod(cachedExecPath, 0755); err != nil {
		return "", fmt.Errorf("failed to make cached terraform binary executable: %w", err)
	}

	return cachedExecPath, nil
}

func (tr *TerraformRunner) Init(opts ...tfexec.InitOption) error {
	log.Info("initializing terraform module")

	err := tr.obtainCacheDirLock()
	if err != nil {
		return err
	}
	defer tr.releaseCacheLock()

	shell("ls -lahR " + tr.ProjectDir)
	shell("ls -lahR " + tr.CacheDir)

	return tr.cmd.Init(context.Background(), tr.GetInitOptions()...)
}

func (tr *TerraformRunner) SelectWorkspace(workspace string) error {
	log.WithField("workspace", workspace).Info("selecting workspace")
	if workspace == "" {
		return nil
	}

	spaces, current, err := tr.cmd.WorkspaceList(context.Background())
	if err != nil {
		return err
	}

	// if the current namespace is the same as the desired workspace
	if current == workspace {
		return nil
	}

	if arrayContains(spaces, workspace) {
		if err := tr.cmd.WorkspaceSelect(context.Background(), workspace); err != nil {
			return err
		}
	} else {
		if err := tr.cmd.WorkspaceNew(context.Background(), workspace); err != nil {
			return err
		}
	}

	return nil
}

func (tr *TerraformRunner) Plan(opts ...tfexec.PlanOption) error {
	log.Info("running terraform plan")

	shell("ls -lahR " + tr.ProjectDir)
	shell("ls -lahR " + tr.CacheDir)

	diff, err := tr.cmd.Plan(context.Background(), opts...)
	if err != nil {
		return err
	}

	if diff {
		log.Info("plan detected some changes")
	}

	return nil
}

func (tr *TerraformRunner) Apply(opts ...tfexec.ApplyOption) error {
	log.Info("running terraform apply")

	return tr.cmd.Apply(context.Background(), opts...)
}

func (tr *TerraformRunner) Destroy(opts ...tfexec.DestroyOption) error {
	log.Info("running terraform destroy")

	return tr.cmd.Destroy(context.Background(), opts...)
}

func (tr *TerraformRunner) GetOutputs() (map[string][]byte, error) {
	log.Info("retrieving outputs for module")

	outputs, err := tr.cmd.Output(context.Background())

	if err != nil {
		return nil, err
	}

	result := map[string][]byte{}

	for key, o := range outputs {
		result[key] = []byte(string(o.Value))
	}

	return result, nil
}

func (tr *TerraformRunner) GetInitOptions() []tfexec.InitOption {
	opts := []tfexec.InitOption{}

	opts = append(opts, tfexec.Upgrade(true))

	return opts
}

func (tr *TerraformRunner) GetPlanOptions() []tfexec.PlanOption {
	opts := []tfexec.PlanOption{}

	for _, path := range tr.varFiles {
		opts = append(opts, tfexec.VarFile(path))
	}

	opts = append(opts, tfexec.Out("/tmp/tf-plan"))

	return opts
}

func (tr *TerraformRunner) GetApplyOptions() []tfexec.ApplyOption {
	opts := []tfexec.ApplyOption{}

	for _, path := range tr.varFiles {
		opts = append(opts, tfexec.VarFile(path))
	}

	return opts
}

func (tr *TerraformRunner) GetDestroyOptions() []tfexec.DestroyOption {
	opts := []tfexec.DestroyOption{}

	for _, path := range tr.varFiles {
		opts = append(opts, tfexec.VarFile(path))
	}

	return opts
}

func (tr *TerraformRunner) obtainCacheDirLock() error {
	log := tr.log.WithField("cacheLock", tr.cacheLock.Path())

	// Set timeout for acquiring lock
	ctx, cancel := context.WithTimeout(context.Background(), terraformCacheLockTimeout)
	defer cancel()

	log.Info("attempting to acquire")

	// Try to acquire the lock with timeout
	locked, err := tr.cacheLock.TryLockContext(ctx, terraformCacheLockPoll)
	if err != nil {
		return fmt.Errorf("failed to acquire cache lock: %w", err)
	}
	if !locked {
		return fmt.Errorf("timeout waiting for cache lock: %v", terraformCacheLockTimeout)
	}

	log.Info("acquired")
	return nil
}

func (tr *TerraformRunner) releaseCacheLock() error {
	if err := tr.cacheLock.Unlock(); err != nil {
		tr.log.WithError(err).Error("filed to release cache lock")
		return err
	}

	log.Info("released cache lock")
	return nil
}
