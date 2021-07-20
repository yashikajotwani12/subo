package command

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/suborbital/subo/builder/context"
	"github.com/suborbital/subo/builder/template"
	"github.com/suborbital/subo/subo/input"
	"github.com/suborbital/subo/subo/util"
)

type deployData struct {
	SCCVersion       string
	EnvToken         string
	BuilderDomain    string
	StorageClassName string
}

// ComputeDeployCoreCommand returns the compute deploy command
func ComputeDeployCoreCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "core",
		Short: "deploy the Suborbital Compute Core",
		Long:  `deploy the Suborbital Compute Core using Kubernetes or Docker Compose`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := introAcceptance(); err != nil {
				return err
			}

			cwd, err := os.Getwd()
			if err != nil {
				return errors.Wrap(err, "failed to Getwd")
			}

			bctx, err := context.ForDirectory(cwd)
			if err != nil {
				return errors.Wrap(err, "🚫 failed to get CurrentBuildContext")
			}

			util.LogStart("preparing deployment")

			// start with a clean slate
			if _, err := os.Stat(filepath.Join(bctx.Cwd, ".suborbital")); err == nil {
				if err := os.RemoveAll(filepath.Join(bctx.Cwd, ".suborbital")); err != nil {
					return errors.Wrap(err, "failed to RemoveAll")
				}
			}

			_, err = util.Mkdir(bctx.Cwd, ".suborbital")
			if err != nil {
				return errors.Wrap(err, "🚫 failed to Mkdir")
			}

			branch, _ := cmd.Flags().GetString(branchFlag)

			templatesPath, err := template.UpdateTemplates(defaultRepo, branch)
			if err != nil {
				return errors.Wrap(err, "🚫 failed to UpdateTemplates")
			}

			envToken, err := getEnvToken()
			if err != nil {
				return errors.Wrap(err, "🚫 failed to getEnvToken")
			}

			builderDomain, err := getBuilderDomain()
			if err != nil {
				return errors.Wrap(err, "🚫 failed to getBuilderDomain")
			}

			storageClass, err := getStorageClass()
			if err != nil {
				return errors.Wrap(err, "🚫 failed to getStorageClass")
			}

			data := deployData{
				SCCVersion:       "latest",
				EnvToken:         envToken,
				BuilderDomain:    builderDomain,
				StorageClassName: storageClass,
			}

			if err := template.ExecTmplDir(bctx.Cwd, ".suborbital", templatesPath, "scc-k8s", data); err != nil {
				return errors.Wrap(err, "🚫 failed to ExecTmplDir")
			}

			util.LogDone("ready to start installation")

			dryRun, _ := cmd.Flags().GetBool(dryRunFlag)

			if !dryRun {
				util.LogStart("installing...")

				// we don't care if this fails, so don't check error
				util.Run("kubectl create ns suborbital")

				if _, err := util.Run("kubectl apply -f .suborbital/"); err != nil {
					return errors.Wrap(err, "🚫 failed to kubectl apply")
				}

				util.LogDone("installation complete!")
				util.LogInfo("use `kubectl get pods -n suborbital` and `kubectl get svc -n suborbital` to check deployment status")
			} else {
				util.LogInfo("aborting due to dry-run, manifest files left in .suborbital")
			}

			return nil
		},
	}

	cmd.Flags().String(branchFlag, "main", "git branch to download templates from")
	cmd.Flags().Bool(dryRunFlag, false, "prepare the installation in the .suborbital directory, but do not apply it")

	return cmd
}

func introAcceptance() error {
	fmt.Print(`
Suborbital Compute Core Installer

BEFORE YOU CONTINUE:
	- You must first run "subo compute create token <email>" to get an environment token

	- You must have kubectl installed in PATH, and it must be connected to the cluster you'd like to use

	- You must be able to set up DNS records for the builder service after this installation completes
			- Choose the DNS name you'd like to use before continuing, e.g. builder.acmeco.com

	- You must know the correct Kubernetes storage class for your cluster (this varies by cloud provider)
		- See the Flight Deck documentation for more details

Are you ready to continue? (y/N): `)

	answer, err := input.ReadStdinString()
	if err != nil {
		return errors.Wrap(err, "failed to ReadStdinString")
	}

	if !strings.EqualFold(answer, "y") {
		return errors.New("aborting")
	}

	return nil
}

// getEnvToken gets the environment token from stdin
func getEnvToken() (string, error) {
	fmt.Print("Enter your environment token: ")
	token, err := input.ReadStdinString()
	if err != nil {
		return "", errors.Wrap(err, "failed to ReadStdinString")
	}

	if len(token) != 32 {
		return "", errors.New("token must be 32 characters in length")
	}

	return token, nil
}

// getBuilderDomain gets the environment token from stdin
func getBuilderDomain() (string, error) {
	fmt.Print("Enter the domain name that will be used for the builder service: ")
	domain, err := input.ReadStdinString()
	if err != nil {
		return "", errors.Wrap(err, "failed to ReadStdinString")
	}

	if len(domain) == 0 {
		return "", errors.New("domain must not be empty")
	}

	return domain, nil
}

// getStorageClass gets the storage class to use
func getStorageClass() (string, error) {
	fmt.Print("Enter the Kubernetes storage class to use: ")
	storageClass, err := input.ReadStdinString()
	if err != nil {
		return "", errors.Wrap(err, "failed to ReadStdinString")
	}

	if len(storageClass) == 0 {
		return "", errors.New("storage class must not be empty")
	}

	return storageClass, nil
}
