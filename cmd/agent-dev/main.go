package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pinpt/agent.next/cmd/agent-dev/cmdbuild"
	"github.com/pinpt/agent.next/cmd/cmdupload"
	"github.com/pinpt/agent.next/integrations/pkg/commiturl"
	"github.com/pinpt/agent.next/integrations/pkg/commonrepo"

	"github.com/pinpt/agent.next/pkg/expsessions"
	"github.com/pinpt/agent.next/pkg/fsconf"
	"github.com/pinpt/agent.next/pkg/jsonstore"

	"github.com/pinpt/agent.next/pkg/exportrepo"
	"github.com/pinpt/agent.next/pkg/gitclone"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/pinpt/go-common/hash"
	"github.com/spf13/cobra"
)

var cmdRoot = &cobra.Command{
	Use:              "agent-dev",
	Long:             "agent-dev",
	TraverseChildren: true,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func defaultLogger() hclog.Logger {
	return hclog.New(&hclog.LoggerOptions{
		Output:     os.Stdout,
		Level:      hclog.Debug,
		JSONFormat: false,
	})
}

func exitWithErr(logger hclog.Logger, err error) {
	logger.Error("error: " + err.Error())
	os.Exit(1)
}

var cmdID = &cobra.Command{
	Use:   "id",
	Short: "Create id hash from passed params",
	Args:  cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		var args2 []interface{}
		for _, arg := range args {
			args2 = append(args2, arg)
		}
		fmt.Println(hash.Values(args2...))
	},
}

func init() {
	cmdRoot.AddCommand(cmdID)
}

var cmdCloneRepo = &cobra.Command{
	Use:   "clone-repo",
	Short: "Clone the repo",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		logger := defaultLogger()

		ctx := context.Background()
		url, _ := cmd.Flags().GetString("url")
		cacheDir, _ := cmd.Flags().GetString("cache-dir")
		checkoutDir, _ := cmd.Flags().GetString("checkout-dir")
		res, err := gitclone.CloneWithCache(ctx, logger, gitclone.AccessDetails{
			URL: url,
		}, gitclone.Dirs{
			CacheRoot: cacheDir,
			Checkout:  checkoutDir,
		}, "1", "main-repo")
		fmt.Println("res", res)
		if err != nil {
			exitWithErr(logger, err)
		}
	},
}

func init() {
	cmdCloneRepo.Flags().String("url", "", "repo url")
	cmdCloneRepo.Flags().String("cache-dir", "", "cache-dir for repos")
	cmdCloneRepo.Flags().String("checkout-dir", "", "checkout-dir")
	cmdRoot.AddCommand(cmdCloneRepo)
}

var cmdExportRepo = &cobra.Command{
	Use:   "export-repo",
	Short: "Clone the repo and run ripsrc and write the output",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		started := time.Now()
		logger := defaultLogger()
		ctx := context.Background()
		url, _ := cmd.Flags().GetString("url")
		pinpointRoot, _ := cmd.Flags().GetString("pinpoint-root")
		if pinpointRoot == "" {
			panic("provide pinpoint-root")
		}
		locs := fsconf.New(pinpointRoot)

		lastProcessed, err := jsonstore.New(locs.LastProcessedFile)
		if err != nil {
			panic(err)
		}

		sessions := expsessions.New(expsessions.Opts{
			Logger:        logger,
			LastProcessed: lastProcessed,
			NewWriter: func(modelName string, id expsessions.ID) expsessions.Writer {
				return expsessions.NewFileWriter(modelName, locs.Uploads, id)
			},
		})

		repoName, _ := cmd.Flags().GetString("repo-name")
		dummyRepo := commonrepo.Repo{}
		if repoName != "" {
			dummyRepo.NameWithOwner = repoName
		} else {
			dummyRepo.NameWithOwner = strings.Replace(filepath.Base(url), ".git", "", 1)
		}
		reftype, _ := cmd.Flags().GetString("ref-type")

		opts := exportrepo.Opts{
			Logger:            logger,
			RepoAccess:        gitclone.AccessDetails{URL: url},
			Sessions:          sessions,
			RepoID:            "r1",
			UniqueName:        "repo1",
			CustomerID:        "c1",
			LastProcessed:     lastProcessed,
			CommitURLTemplate: commiturl.CommitURLTemplate(dummyRepo, url),
			BranchURLTemplate: commiturl.BranchURLTemplate(dummyRepo, url),
			RefType:           reftype,
		}

		exp := exportrepo.New(opts, locs)
		res := exp.Run(ctx)
		if res.SessionErr != nil {
			exitWithErr(logger, fmt.Errorf("session err: %v", err))
		}
		if res.OtherErr != nil {
			exitWithErr(logger, fmt.Errorf("other err: %v", err))
		}
		if err := lastProcessed.Save(); err != nil {
			exitWithErr(logger, err)
		}
		dur := res.Duration
		logger.Info("export-repo completed", "duration", time.Since(started), "gitclone", dur.Clone.String(), "ripsrc", dur.Ripsrc.String())

	},
}

func init() {
	cmdExportRepo.Flags().String("url", "", "repo url")
	cmdExportRepo.Flags().String("pinpoint-root", "", "pinpoint-root")
	cmdExportRepo.Flags().String("ref-type", "git", "ref-type")
	cmdExportRepo.Flags().String("repo-name", "", "repo-name")
	cmdRoot.AddCommand(cmdExportRepo)
}

func flagPinpointRoot(cmd *cobra.Command) {
	cmd.Flags().String("pinpoint-root", "", "Custom location of pinpoint work dir.")
}

func getPinpointRoot(cmd *cobra.Command) (string, error) {
	res, _ := cmd.Flags().GetString("pinpoint-root")
	if res != "" {
		return res, nil
	}
	return fsconf.DefaultRoot()
}

var cmdUpload = &cobra.Command{
	Use:   "upload <upload_url> <api_key>",
	Short: "Upload processed data",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		logger := defaultLogger()
		ctx := context.Background()

		uploadURL := args[0]
		apiKey := args[1]

		pinpointRoot, err := getPinpointRoot(cmd)
		if err != nil {
			exitWithErr(logger, err)
		}

		_, _, err = cmdupload.Run(ctx, logger, pinpointRoot, uploadURL, apiKey)
		if err != nil {
			exitWithErr(logger, err)
		}
	},
}

func init() {
	cmd := cmdUpload
	flagPinpointRoot(cmd)
	cmdRoot.AddCommand(cmd)
}

var cmdBuild = &cobra.Command{
	Use:   "build",
	Short: "Build agent and integrations and create a release",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		version, _ := cmd.Flags().GetString("version")
		upload, _ := cmd.Flags().GetBool("upload")
		platform, _ := cmd.Flags().GetString("platform")
		if platform == "all" {
			platform = ""
		}
		onlyAgent, _ := cmd.Flags().GetBool("only-agent")
		onlyUpload, _ := cmd.Flags().GetBool("only-upload")

		cmdbuild.Run(cmdbuild.Opts{
			BuildDir:     "./dist",
			Version:      version,
			Upload:       upload,
			OnlyUpload:   onlyUpload,
			OnlyPlatform: platform,
			OnlyAgent:    onlyAgent,
		})
	},
}

func init() {
	cmd := cmdBuild
	cmd.Flags().String("version", "test", "Version to use for release")
	cmd.Flags().Bool("upload", false, "Set to true to upload release to S3")
	cmd.Flags().Bool("only-upload", false, "Set to true to skip build and upload existing files in dist dir")
	cmd.Flags().String("platform", "all", "Limit to specific platform")
	cmd.Flags().Bool("only-agent", false, "Only build agent and skip the rest (for developement)")
	cmdRoot.AddCommand(cmd)
}

func main() {
	cmdRoot.Execute()
}
