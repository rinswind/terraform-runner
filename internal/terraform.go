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

type TerraformRunner struct {
	CMD *tfexec.Terraform

	ProjectDir string
	CacheDir   string
	VarFiles   []string

	log *log.Entry
}

func Setup() (*TerraformRunner, error) {
	log := log.WithField("version", Env.TerraformVersion).WithField("project_dir", Env.ProjectDir)

	tr := &TerraformRunner{
		ProjectDir: Env.ProjectDir,
		CacheDir:   Env.PluginCache,
		log:        log,
	}

	// Install terraform binary
	execPath, err := tr.Install()
	if err != nil {
		tr.log.WithField("error", err).Error("error installing Terraform")
		return nil, err
	}

	// Setup a terraform exec
	cmd, err := tfexec.NewTerraform(Env.ProjectDir, execPath)
	if err != nil {
		tr.log.WithField("error", err).Error("error running NewTerraform")
		return nil, err
	}

	cmd.SetLogger(tr.log)

	tr.CMD = cmd

	// Find the var files
	varFiles, err := getTfVarFilesPaths(Env.VarFilesPath)
	if err != nil {
		tr.log.WithField("error", err).Error("failed to list files in the var files path")
		return nil, err
	}

	tr.VarFiles = varFiles

	return tr, nil
}

func (tr *TerraformRunner) Install() (string, error) {
	flock, err := tr.obtainCacheDirLock()
	if err != nil {
		return "", err
	}
	defer releaseFileLock(flock)

	cachedExecPath := filepath.Join(tr.CacheDir, fmt.Sprintf("terraform-%s", Env.TerraformVersion))

	// Check whether there's a file at cachedExecPath
	if info, err := os.Stat(cachedExecPath); err == nil {
		tr.log.WithField("path", cachedExecPath).Info("made cached terraform binary executable")

		// If it's not executable make it so - just in case
		if mode := info.Mode(); mode&0111 != 0 {
			return cachedExecPath, nil
		}

		if err := os.Chmod(cachedExecPath, 0755); err != nil {
			return "", fmt.Errorf("failed to make cached terraform binary executable: %w", err)
		}

		return cachedExecPath, nil
	}

	// Install terraform to the working_dir
	tr.log.Info("installing terraform")

	installer := &releases.ExactVersion{
		Product:    product.Terraform,
		Version:    version.Must(version.NewVersion(Env.TerraformVersion)),
		InstallDir: tr.CacheDir,
	}

	execPath, err := installer.Install(context.Background())
	if err != nil {
		return "", err
	}

	if err := os.Rename(execPath, cachedExecPath); err != nil {
		return "", fmt.Errorf("failed to move terraform binary to cache dir: %w", err)
	}

	return cachedExecPath, nil
}

func (tr *TerraformRunner) Init() error {
	log.Info("initializing terraform module")

	flock, err := tr.obtainCacheDirLock()
	if err != nil {
		return err
	}
	defer releaseFileLock(flock)

	return tr.CMD.Init(context.Background(), tr.GetInitOptions()...)
}

func (tr *TerraformRunner) SelectWorkspace(workspace string) error {
	log.WithField("workspace", workspace).Info("selecting workspace")
	if workspace == "" {
		return nil
	}

	spaces, current, err := tr.CMD.WorkspaceList(context.Background())
	if err != nil {
		return err
	}

	// if the current namespace is the same as the desired workspace
	if current == workspace {
		return nil
	}

	if arrayContains(spaces, workspace) {
		if err := tr.CMD.WorkspaceSelect(context.Background(), workspace); err != nil {
			return err
		}
	} else {
		if err := tr.CMD.WorkspaceNew(context.Background(), workspace); err != nil {
			return err
		}
	}

	return nil
}

func (tr *TerraformRunner) Plan(opts ...tfexec.PlanOption) error {
	log.Info("running terraform plan")

	diff, err := tr.CMD.Plan(context.Background(), opts...)
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

	return tr.CMD.Apply(context.Background(), opts...)
}

func (tr *TerraformRunner) Destroy(opts ...tfexec.DestroyOption) error {
	log.Info("running terraform destroy")

	return tr.CMD.Destroy(context.Background(), opts...)
}

func (tr *TerraformRunner) GetOutputs() (map[string][]byte, error) {
	log.Info("retrieving outputs for module")

	outputs, err := tr.CMD.Output(context.Background())

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

	for _, path := range tr.VarFiles {
		opts = append(opts, tfexec.VarFile(path))
	}

	opts = append(opts, tfexec.Out("/tmp/tfplan"))

	return opts
}

func (tr *TerraformRunner) GetApplyOptions() []tfexec.ApplyOption {
	opts := []tfexec.ApplyOption{}

	for _, path := range tr.VarFiles {
		opts = append(opts, tfexec.VarFile(path))
	}

	return opts
}

func (tr *TerraformRunner) GetDestroyOptions() []tfexec.DestroyOption {
	opts := []tfexec.DestroyOption{}

	for _, path := range tr.VarFiles {
		opts = append(opts, tfexec.VarFile(path))
	}

	return opts
}

func (tr *TerraformRunner) obtainCacheDirLock() (*flock.Flock, error) {
	// Ensure cache directory exists
	if err := os.MkdirAll(tr.CacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory %s: %w", tr.CacheDir, err)
	}

	// Create lock file path
	lockPath := filepath.Join(tr.CacheDir, ".terraform-init.lock")
	fileLock := flock.New(lockPath)

	// Set timeout for acquiring lock
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	log.WithField("lockPath", lockPath).Info("Attempting to acquire terraform init lock")

	// Try to acquire the lock with timeout
	locked, err := fileLock.TryLockContext(ctx, 100*time.Millisecond) // retry every 100ms
	if err != nil {
		return nil, fmt.Errorf("failed to acquire terraform init lock: %w", err)
	}
	if !locked {
		return nil, fmt.Errorf("timeout waiting for terraform init lock after 5 minutes")
	}

	log.Info("Acquired terraform init lock, proceeding with init")
	return fileLock, nil
}

func releaseFileLock(fileLock *flock.Flock) {
	if err := fileLock.Unlock(); err != nil {
		log.WithError(err).Error("Failed to release terraform init lock")
	} else {
		log.Info("Released terraform init lock")
	}
}
