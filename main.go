package main

import (
	"encoding/json"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/urfave/cli"
)

var revision string // build number set at compile-time

func main() {
	app := cli.NewApp()
	app.Name = "terraform plugin"
	app.Usage = "terraform plugin"
	app.Action = run
	app.Version = revision
	app.Flags = []cli.Flag{

		//
		// plugin args
		//
		cli.StringFlag{
			Name:   "api-key",
			Usage:  "api key to access github api",
			EnvVar: "PLUGIN_API_KEY,GITHUB_RELEASE_API_KEY,GITHUB_TOKEN",
		},
		cli.StringFlag{
			Name:   "username",
			Usage:  "basic auth username",
			EnvVar: "PLUGIN_USERNAME,GITHUB_USERNAME,DRONE_NETRC_USERNAME",
		},
		cli.StringFlag{
			Name:   "password",
			Usage:  "basic auth password",
			EnvVar: "PLUGIN_PASSWORD,GITHUB_PASSWORD,DRONE_NETRC_PASSWORD",
		},
		cli.StringFlag{
			Name:   "base-url",
			Value:  "https://api.github.com/",
			Usage:  "api url, needs to be changed for ghe",
			EnvVar: "PLUGIN_BASE_URL,GITHUB_BASE_URL",
		},
		cli.StringFlag{
			Name:   "title",
			Value:  "Terraform Plan Output",
			Usage:  "title for comment",
			EnvVar: "PLUGIN_TITLE",
		},
		cli.StringFlag{
			Name:   "mode",
			Value:  "full",
			Usage:  "comment mode [summary, simple, full]",
			EnvVar: "PLUGIN_MODE",
		},
		cli.IntFlag{
			Name:   "issue-num",
			Usage:  "Issue #",
			EnvVar: "PLUGIN_ISSUE_NUM,DRONE_PULL_REQUEST",
		},
		cli.BoolFlag{
			Name:   "recreate",
			Usage:  "recreate the comment every time",
			EnvVar: "PLUGIN_RECREATE",
		},
		cli.StringFlag{
			Name:   "tf_root_dir",
			Usage:  "The root directory where the terraform files live. When unset, the top level directory will be assumed",
			EnvVar: "PLUGIN_ROOT_DIR",
		},
		cli.StringFlag{
			Name:   "tf.version",
			Usage:  "terraform version to use",
			EnvVar: "PLUGIN_TF_VERSION",
		},
		cli.StringFlag{
			Name:   "tf_data_dir",
			Value:  ".terraform",
			Usage:  "changes the location where Terraform keeps its per-working-directory data, such as the current remote backend configuration",
			EnvVar: "PLUGIN_TF_DATA_DIR",
		},
		cli.StringFlag{
			Name:   "ca_cert",
			Usage:  "ca cert to add to your environment to allow terraform to use internal/private resources",
			EnvVar: "PLUGIN_CA_CERT",
		},
		cli.StringFlag{
			Name:   "init_options",
			Usage:  "options for the init command. See https://www.terraform.io/docs/commands/init.html",
			EnvVar: "PLUGIN_INIT_OPTIONS",
		},
		cli.BoolFlag{
			Name:   "debug",
			Usage:  "whether or not to show terraform commands to stdout",
			EnvVar: "PLUGIN_DEBUG",
		},
		cli.StringFlag{
			Name:   "role_arn_to_assume",
			Usage:  "A role to assume before running the terraform commands",
			EnvVar: "PLUGIN_ROLE_ARN_TO_ASSUME",
		},

		//
		// drone env
		//

		cli.StringFlag{
			Name:   "repo-name",
			Usage:  "repository name",
			EnvVar: "DRONE_REPO_NAME",
		},
		cli.StringFlag{
			Name:   "repo-owner",
			Usage:  "repository owner",
			EnvVar: "DRONE_REPO_OWNER",
		},
		cli.StringFlag{
			Name:   "commit-sha",
			Usage:  "git commit SHA",
			EnvVar: "DRONE_COMMIT_SHA",
		},

		//
		// netrc env
		//
		cli.StringFlag{
			Name:   "netrc.machine",
			Usage:  "netrc machine",
			EnvVar: "DRONE_NETRC_MACHINE",
		},
		cli.StringFlag{
			Name:   "netrc.username",
			Usage:  "netrc username",
			EnvVar: "DRONE_NETRC_USERNAME",
		},
		cli.StringFlag{
			Name:   "netrc.password",
			Usage:  "netrc password",
			EnvVar: "DRONE_NETRC_PASSWORD",
		},
	}

	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}

func run(c *cli.Context) error {
	logrus.WithFields(logrus.Fields{
		"Revision": revision,
	}).Info("Drone Terraform GitHub Commenter Plugin Version")

	initOptions := InitOptions{}
	json.Unmarshal([]byte(c.String("init_options")), &initOptions)

	plugin := Plugin{
		Config: Config{
			BaseURL:          c.String("base-url"),
			Mode:             c.String("mode"),
			Title:            c.String("title"),
			IssueNum:         c.Int("issue-num"),
			Password:         c.String("password"),
			RepoName:         c.String("repo-name"),
			RepoOwner:        c.String("repo-owner"),
			CommitSha:        c.String("commit-sha"),
			Token:            c.String("api-key"),
			Recreate:         c.Bool("recreate"),
			Username:         c.String("username"),
			InitOptions:      initOptions,
			Cacert:           c.String("ca_cert"),
			Debug:            c.Bool("debug"),
			RoleARN:          c.String("role_arn_to_assume"),
			TerraformRootDir: c.String("tf_root_dir"),
			TerraformDataDir: c.String("tf_data_dir"),
		},
		Netrc: Netrc{
			Login:    c.String("netrc.username"),
			Machine:  c.String("netrc.machine"),
			Password: c.String("netrc.password"),
		},
		Terraform: Terraform{
			Version: c.String("tf.version"),
		},
	}

	return plugin.Exec()
}
