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
	ProjectDir string
	VarFiles   []string
	CMD        *tfexec.Terraform
}

func Setup() (*TerraformRunner, error) {
	logger := log.WithField("version", Env.TerraformVersion).WithField("project_dir", Env.ProjectDir)

	// Install terraform to the working_dir
	logger.Info("installing terraform version")

	installer := &releases.ExactVersion{
		Product: product.Terraform,
		Version: version.Must(version.NewVersion(Env.TerraformVersion)),

		InstallDir: Env.ProjectDir,
	}

	execPath, err := installer.Install(context.Background())
	if err != nil {
		logger.WithField("error", err).Error("error installing Terraform")
		return nil, err
	}

	// Find the var files
	varFiles, err := getTfVarFilesPaths(Env.VarFilesPath)
	if err != nil {
		logger.WithField("error", err).Error("failed to list files in the var files path")
		return nil, err
	}

	// Setup a terraform exec
	tf, err := tfexec.NewTerraform(Env.ProjectDir, execPath)
	if err != nil {
		logger.WithField("error", err).Error("error running NewTerraform")
		return nil, err
	}

	tf.SetLogger(logger)

	return &TerraformRunner{
		CMD:        tf,
		ProjectDir: Env.ProjectDir,
		VarFiles:   varFiles,
	}, nil
}

func (r *TerraformRunner) Init() error {
	log.Info("initializing terraform module")

	if err := r.CMD.Init(context.Background(), tfexec.Upgrade(true)); err != nil {
		return err
	}

	return nil
}

func (tr *TerraformRunner) CachingInit(cacheDir string) error {
	// Ensure cache directory exists
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory %s: %w", cacheDir, err)
	}

	// Create lock file path
	lockPath := filepath.Join(cacheDir, ".terraform-init.lock")
	fileLock := flock.New(lockPath)

	// Set timeout for acquiring lock
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	log.WithField("lockPath", lockPath).Info("Attempting to acquire terraform init lock")

	// Try to acquire the lock with timeout
	locked, err := fileLock.TryLockContext(ctx, 100*time.Millisecond) // retry every 100ms
	if err != nil {
		return fmt.Errorf("failed to acquire terraform init lock: %w", err)
	}
	if !locked {
		return fmt.Errorf("timeout waiting for terraform init lock after 5 minutes")
	}

	defer func() {
		if err := fileLock.Unlock(); err != nil {
			log.WithError(err).Error("Failed to release terraform init lock")
		} else {
			log.Info("Released terraform init lock")
		}
	}()

	log.Info("Acquired terraform init lock, proceeding with init")
	return tr.Init()
}

func (r *TerraformRunner) SelectWorkspace(workspace string) error {
	log.WithField("workspace", workspace).Info("selecting workspace")
	if workspace == "" {
		return nil
	}

	spaces, current, err := r.CMD.WorkspaceList(context.Background())
	if err != nil {
		return err
	}

	// if the current namespace is the same as the desired workspace
	if current == workspace {
		return nil
	}

	if arrayContains(spaces, workspace) {
		if err := r.CMD.WorkspaceSelect(context.Background(), workspace); err != nil {
			return err
		}
	} else {
		if err := r.CMD.WorkspaceNew(context.Background(), workspace); err != nil {
			return err
		}
	}

	return nil
}

func (r *TerraformRunner) Apply(opts ...tfexec.ApplyOption) error {
	log.Info("running terraform apply")

	return r.CMD.Apply(context.Background(), opts...)
}

func (r *TerraformRunner) Plan(opts ...tfexec.PlanOption) error {
	log.Info("running terraform plan")

	diff, err := r.CMD.Plan(context.Background(), opts...)
	if err != nil {
		return err
	}

	if diff {
		log.Info("plan detected some changes")
	}

	return nil
}

func (r *TerraformRunner) Destroy(opts ...tfexec.DestroyOption) error {
	log.Info("running terraform destroy")

	return r.CMD.Destroy(context.Background(), opts...)
}

func (r *TerraformRunner) GetOutputs() (map[string][]byte, error) {
	log.Info("retrieving outputs for module")

	outputs, err := r.CMD.Output(context.Background())

	if err != nil {
		return nil, err
	}

	result := map[string][]byte{}

	for key, o := range outputs {
		result[key] = []byte(string(o.Value))
	}

	return result, nil
}

func (r *TerraformRunner) GetPlanOptions() []tfexec.PlanOption {
	opts := []tfexec.PlanOption{}

	for _, path := range r.VarFiles {
		opts = append(opts, tfexec.VarFile(path))
	}

	opts = append(opts, tfexec.Out("/tmp/tfplan"))

	return opts
}

func (r *TerraformRunner) GetApplyOptions() []tfexec.ApplyOption {
	opts := []tfexec.ApplyOption{}

	for _, path := range r.VarFiles {
		opts = append(opts, tfexec.VarFile(path))
	}

	return opts
}

func (r *TerraformRunner) GetDestroyOptions() []tfexec.DestroyOption {
	opts := []tfexec.DestroyOption{}

	for _, path := range r.VarFiles {
		opts = append(opts, tfexec.VarFile(path))
	}

	return opts
}
