package get

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/MakeNowJust/heredoc"
	"github.com/cli/cli/v2/api"
	"github.com/cli/cli/v2/internal/gh"
	"github.com/cli/cli/v2/internal/ghrepo"
	"github.com/cli/cli/v2/pkg/cmd/variable/shared"
	"github.com/cli/cli/v2/pkg/cmdutil"
	"github.com/cli/cli/v2/pkg/iostreams"
	"github.com/spf13/cobra"
)

type GetOptions struct {
	HttpClient func() (*http.Client, error)
	IO         *iostreams.IOStreams
	Config     func() (gh.Config, error)
	BaseRepo   func() (ghrepo.Interface, error)

	VariableName string
	OrgName      string
	EnvName      string
}

type getVariableResponse struct {
	Value string `json:"value"`
	// Other available but unused fields
	// Name             string            `json:"name"`
	// UpdatedAt        time.Time         `json:"updated_at"`
	// Visibility       shared.Visibility `json:"visibility"`
	// SelectedReposURL string            `json:"selected_repositories_url"`
	// NumSelectedRepos int               `json:"num_selected_repos"`
}

func NewCmdGet(f *cmdutil.Factory, runF func(*GetOptions) error) *cobra.Command {
	opts := &GetOptions{
		IO:         f.IOStreams,
		Config:     f.Config,
		HttpClient: f.HttpClient,
	}

	cmd := &cobra.Command{
		Use:   "get <variable-name>",
		Short: "Get variables",
		Long: heredoc.Doc(`
			Get a variable on one of the following levels:
			- repository (default): available to GitHub Actions runs or Dependabot in a repository
			- environment: available to GitHub Actions runs for a deployment environment in a repository
			- organization: available to GitHub Actions runs or Dependabot within an organization
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// support `-R, --repo` override
			opts.BaseRepo = f.BaseRepo

			if err := cmdutil.MutuallyExclusive("specify only one of `--org` or `--env`", opts.OrgName != "", opts.EnvName != ""); err != nil {
				return err
			}

			opts.VariableName = args[0]

			if runF != nil {
				return runF(opts)
			}

			return getRun(opts)
		},
	}
	cmd.Flags().StringVarP(&opts.OrgName, "org", "o", "", "Get a variable for an organization")
	cmd.Flags().StringVarP(&opts.EnvName, "env", "e", "", "Get a variable for an environment")

	return cmd
}

func getRun(opts *GetOptions) error {
	c, err := opts.HttpClient()
	if err != nil {
		return fmt.Errorf("could not create http client: %w", err)
	}
	client := api.NewClientFromHTTP(c)

	orgName := opts.OrgName
	envName := opts.EnvName

	variableEntity, err := shared.GetVariableEntity(orgName, envName)
	if err != nil {
		return err
	}

	var baseRepo ghrepo.Interface
	if variableEntity == shared.Repository || variableEntity == shared.Environment {
		baseRepo, err = opts.BaseRepo()
		if err != nil {
			return err
		}
	}

	var path string
	switch variableEntity {
	case shared.Organization:
		path = fmt.Sprintf("orgs/%s/actions/variables/%s", orgName, opts.VariableName)
	case shared.Environment:
		path = fmt.Sprintf("repos/%s/environments/%s/variables/%s", ghrepo.FullName(baseRepo), envName, opts.VariableName)
	case shared.Repository:
		path = fmt.Sprintf("repos/%s/actions/variables/%s", ghrepo.FullName(baseRepo), opts.VariableName)
	}

	cfg, err := opts.Config()
	if err != nil {
		return err
	}

	host, _ := cfg.Authentication().DefaultHost()

	var response getVariableResponse
	if err = client.REST(host, "GET", path, nil, &response); err != nil {
		var httpErr api.HTTPError
		if errors.As(err, &httpErr) && httpErr.StatusCode == http.StatusNotFound {
			return fmt.Errorf("variable %s was not found", opts.VariableName)
		}

		return fmt.Errorf("failed to get variable %s: %w", opts.VariableName, err)
	}

	fmt.Fprintf(opts.IO.Out, "%s\n", response.Value)

	return nil
}
