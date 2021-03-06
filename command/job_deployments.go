package command

import (
	"fmt"
	"strings"
)

type JobDeploymentsCommand struct {
	Meta
}

func (c *JobDeploymentsCommand) Help() string {
	helpText := `
Usage: nomad job deployments [options] <job>

Deployments is used to display the deployments for a particular job.

General Options:

  ` + generalOptionsUsage() + `

Deployments Options:

  -json
    Output the deployments in a JSON format.

  -t
    Format and display deployments using a Go template.

  -latest
    Display the latest deployment only.

  -verbose
    Display full information.
`
	return strings.TrimSpace(helpText)
}

func (c *JobDeploymentsCommand) Synopsis() string {
	return "List deployments for a job"
}

func (c *JobDeploymentsCommand) Run(args []string) int {
	var json, latest, verbose bool
	var tmpl string

	flags := c.Meta.FlagSet("job deployments", FlagSetClient)
	flags.Usage = func() { c.Ui.Output(c.Help()) }
	flags.BoolVar(&latest, "latest", false, "")
	flags.BoolVar(&verbose, "verbose", false, "")
	flags.BoolVar(&json, "json", false, "")
	flags.StringVar(&tmpl, "t", "", "")

	if err := flags.Parse(args); err != nil {
		return 1
	}

	// Check that we got exactly one node
	args = flags.Args()
	if l := len(args); l != 1 {
		c.Ui.Error(c.Help())
		return 1
	}

	// Get the HTTP client
	client, err := c.Meta.Client()
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error initializing client: %s", err))
		return 1
	}

	jobID := args[0]

	// Check if the job exists
	jobs, _, err := client.Jobs().PrefixList(jobID)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error listing jobs: %s", err))
		return 1
	}
	if len(jobs) == 0 {
		c.Ui.Error(fmt.Sprintf("No job(s) with prefix or id %q found", jobID))
		return 1
	}
	if len(jobs) > 1 && strings.TrimSpace(jobID) != jobs[0].ID {
		c.Ui.Output(fmt.Sprintf("Prefix matched multiple jobs\n\n%s", createStatusListOutput(jobs)))
		return 0
	}
	jobID = jobs[0].ID

	// Truncate the id unless full length is requested
	length := shortId
	if verbose {
		length = fullId
	}

	if latest {
		deploy, _, err := client.Jobs().LatestDeployment(jobID, nil)
		if err != nil {
			c.Ui.Error(fmt.Sprintf("Error retrieving deployments: %s", err))
			return 1
		}

		c.Ui.Output(c.Colorize().Color(formatDeployment(deploy, length)))
		return 0
	}

	deploys, _, err := client.Jobs().Deployments(jobID, nil)
	if err != nil {
		c.Ui.Error(fmt.Sprintf("Error retrieving deployments: %s", err))
		return 1
	}

	c.Ui.Output(formatDeployments(deploys, length))
	return 0
}
