package main

import (
	"context"
	"fmt"
	"os"

	"github.com/pinpt/agent.next/cmd/cmdupload"

	"github.com/pinpt/agent.next/pkg/expsessions"
	"github.com/pinpt/agent.next/pkg/fsconf"
	"github.com/pinpt/agent.next/pkg/jsonstore"

	"github.com/pinpt/agent.next/pkg/exportrepo"
	"github.com/pinpt/agent.next/pkg/gitclone"

	"github.com/hashicorp/go-hclog"
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
		}, "repo1")
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

		opts := exportrepo.Opts{
			Logger:        logger,
			RepoAccess:    gitclone.AccessDetails{URL: url},
			Sessions:      sessions,
			RepoID:        "r1",
			UniqueName:    "repo1",
			CustomerID:    "c1",
			LastProcessed: lastProcessed,
		}

		exp := exportrepo.New(opts, locs)
		_, err = exp.Run(ctx)
		if err != nil {
			exitWithErr(logger, err)
		}

	},
}

func init() {
	cmdExportRepo.Flags().String("url", "", "repo url")
	cmdExportRepo.Flags().String("pinpoint-root", "", "pinpoint-root")
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
	Use:   "upload <upload_url>",
	Short: "Upload processed data",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		logger := defaultLogger()
		ctx := context.Background()

		uploadURL := args[0]
		pinpointRoot, err := getPinpointRoot(cmd)
		if err != nil {
			exitWithErr(logger, err)
		}

		err = cmdupload.Run(ctx, logger, pinpointRoot, uploadURL)
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

func main() {
	cmdRoot.Execute()
}
