package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pinpt/agent/cmd/agent-dev/cmddownloadexports"
	"github.com/pinpt/agent/cmd/agent-dev/cmddownloadlogs"

	"github.com/pinpt/agent/cmd/agent-dev/cmdbuild"
	"github.com/pinpt/agent/cmd/cmdexport/process"
	"github.com/pinpt/agent/cmd/cmdupload"
	"github.com/pinpt/agent/integrations/pkg/commiturl"
	"github.com/pinpt/agent/integrations/pkg/commonrepo"

	"github.com/pinpt/agent/pkg/encrypt"
	"github.com/pinpt/agent/pkg/expsessions"
	"github.com/pinpt/agent/pkg/fsconf"
	"github.com/pinpt/agent/pkg/jsonstore"

	"github.com/pinpt/agent/pkg/exportrepo"
	"github.com/pinpt/agent/pkg/gitclone"

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

		commitUsers := process.NewCommitUsers()

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
			CommitUsers:       commitUsers,
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

		_, _, err = cmdupload.Run(ctx, logger, pinpointRoot, uploadURL, "jobid1", apiKey, "")
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
		skipArchives, _ := cmd.Flags().GetBool("skip-archives")

		cmdbuild.Run(cmdbuild.Opts{
			BuildDir:     "./dist",
			Version:      version,
			Upload:       upload,
			OnlyUpload:   onlyUpload,
			OnlyPlatform: platform,
			OnlyAgent:    onlyAgent,
			SkipArchives: skipArchives,
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
	cmd.Flags().Bool("skip-archives", false, "Skip creating zips and gzips (faster builds)")
	cmdRoot.AddCommand(cmd)
}

var cmdDownloadLogs = &cobra.Command{
	Use:   "download-logs",
	Short: "Downloads logs from elastic search",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		user, _ := cmd.Flags().GetString("user")
		password, _ := cmd.Flags().GetString("password")
		url, _ := cmd.Flags().GetString("url")
		agentUUID, _ := cmd.Flags().GetString("agent-uuid")
		customerID, _ := cmd.Flags().GetString("customer-id")
		noFormat, _ := cmd.Flags().GetBool("no-format")
		maxRecords, _ := cmd.Flags().GetInt("max-records")
		cmddownloadlogs.Run(cmddownloadlogs.Opts{
			User:       user,
			Password:   password,
			URL:        url,
			AgentUUID:  agentUUID,
			CustomerID: customerID,
			NoFormat:   noFormat,
			MaxRecords: maxRecords,
		})
	},
}

func init() {
	cmd := cmdDownloadLogs
	cmd.Flags().String("user", "", "User")
	cmd.Flags().String("password", "", "Password")
	cmd.Flags().String("url", "", "Elastic search URL")
	cmd.Flags().String("agent-uuid", "", "Agent UUID")
	cmd.Flags().String("customer-id", "", "Customer ID")
	cmd.Flags().Int("max-records", 10000, "Max log records to fetch")
	cmd.Flags().String("no-format", "", "Do not format resulting json (useful to see the exact data returned)")
	cmdRoot.AddCommand(cmd)
}

var cmdDownloadExports = &cobra.Command{
	Use:   "download-exports",
	Short: "Downloads exports from S3",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		channel, _ := cmd.Flags().GetString("channel")
		customerID, _ := cmd.Flags().GetString("customer-id")
		outputDir, _ := cmd.Flags().GetString("output-dir")
		cmddownloadexports.Run(cmddownloadexports.Opts{
			Channel:    channel,
			CustomerID: customerID,
			OutputDir:  outputDir,
		})
	},
}

func init() {
	cmd := cmdDownloadExports
	cmd.Flags().String("channel", "edge", "Channel")
	cmd.Flags().String("customer-id", "", "Customer ID")
	cmd.Flags().String("output-dir", "", "Output dir")
	cmdRoot.AddCommand(cmd)
}

var cmdDecrypt = &cobra.Command{
	Use:   "decrypt",
	Short: "Decrypt authorization message",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		key, _ := cmd.Flags().GetString("key")
		message, _ := cmd.Flags().GetString("message")

		decr, err := encrypt.DecryptString(message, key)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Println(decr)
	},
}

func init() {
	cmd := cmdDecrypt
	cmd.Flags().String("key", "", "Key")
	cmd.Flags().String("message", "", "Message")
	cmdRoot.AddCommand(cmd)
}

func main() {
	cmdRoot.Execute()
}
