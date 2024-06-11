package get

import (
	"bytes"
	"fmt"
	"net/http"
	"testing"

	"github.com/cli/cli/v2/internal/config"
	"github.com/cli/cli/v2/internal/gh"
	"github.com/cli/cli/v2/internal/ghrepo"
	"github.com/cli/cli/v2/pkg/cmdutil"
	"github.com/cli/cli/v2/pkg/httpmock"
	"github.com/cli/cli/v2/pkg/iostreams"
	"github.com/google/shlex"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCmdGet(t *testing.T) {
	tests := []struct {
		name    string
		cli     string
		wants   GetOptions
		wantErr error
	}{
		{
			name: "repo",
			cli:  "FOO",
			wants: GetOptions{
				OrgName:      "",
				VariableName: "FOO",
			},
		},
		{
			name: "org",
			cli:  "-o TestOrg BAR",
			wants: GetOptions{
				OrgName:      "TestOrg",
				VariableName: "BAR",
			},
		},
		{
			name: "env",
			cli:  "-e Development BAZ",
			wants: GetOptions{
				EnvName:      "Development",
				VariableName: "BAZ",
			},
		},
		{
			name:    "org and env",
			cli:     "-o TestOrg -e Development QUX",
			wantErr: cmdutil.FlagErrorf("%s", "specify only one of `--org` or `--env`"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ios, _, _, _ := iostreams.Test()
			f := &cmdutil.Factory{
				IOStreams: ios,
			}

			argv, err := shlex.Split(tt.cli)
			assert.NoError(t, err)

			var gotOpts *GetOptions
			cmd := NewCmdGet(f, func(opts *GetOptions) error {
				gotOpts = opts
				return nil
			})
			cmd.SetArgs(argv)
			cmd.SetIn(&bytes.Buffer{})
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})

			_, err = cmd.ExecuteC()
			if tt.wantErr != nil {
				require.Equal(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)

			require.Equal(t, tt.wants.OrgName, gotOpts.OrgName)
			require.Equal(t, tt.wants.EnvName, gotOpts.EnvName)
			require.Equal(t, tt.wants.VariableName, gotOpts.VariableName)
		})
	}
}

func Test_getRun(t *testing.T) {
	tests := []struct {
		name      string
		opts      *GetOptions
		httpStubs func(*httpmock.Registry)
		wantOut   string
		wantErr   error
	}{
		{
			name: "getting repo variable",
			opts: &GetOptions{
				VariableName: "VARIABLE_ONE",
			},
			httpStubs: func(reg *httpmock.Registry) {
				reg.Register(httpmock.REST("GET", "repos/owner/repo/actions/variables/VARIABLE_ONE"),
					httpmock.JSONResponse(getVariableResponse{
						Value: "repo_var",
					}))
			},
			wantOut: "repo_var\n",
		},
		{
			name: "getting org variable",
			opts: &GetOptions{
				OrgName:      "TestOrg",
				VariableName: "VARIABLE_ONE",
			},
			httpStubs: func(reg *httpmock.Registry) {
				reg.Register(httpmock.REST("GET", "orgs/TestOrg/actions/variables/VARIABLE_ONE"),
					httpmock.JSONResponse(getVariableResponse{
						Value: "org_var",
					}))
			},
			wantOut: "org_var\n",
		},
		{
			name: "getting env variable",
			opts: &GetOptions{
				EnvName:      "Development",
				VariableName: "VARIABLE_ONE",
			},
			httpStubs: func(reg *httpmock.Registry) {
				reg.Register(httpmock.REST("GET", "repos/owner/repo/environments/Development/variables/VARIABLE_ONE"),
					httpmock.JSONResponse(getVariableResponse{
						Value: "env_var",
					}))
			},
			wantOut: "env_var\n",
		},
		{
			name: "when the variable is not found, an error is returned",
			opts: &GetOptions{
				VariableName: "VARIABLE_ONE",
			},
			httpStubs: func(reg *httpmock.Registry) {
				reg.Register(httpmock.REST("GET", "repos/owner/repo/actions/variables/VARIABLE_ONE"),
					httpmock.StatusStringResponse(404, "not found"),
				)
			},
			wantErr: fmt.Errorf("variable VARIABLE_ONE was not found"),
		},
		{
			name: "when getting any variable from API fails, the error is bubbled with context",
			opts: &GetOptions{
				VariableName: "VARIABLE_ONE",
			},
			httpStubs: func(reg *httpmock.Registry) {
				reg.Register(httpmock.REST("GET", "repos/owner/repo/actions/variables/VARIABLE_ONE"),
					httpmock.StatusStringResponse(400, "not found"),
				)
			},
			wantErr: fmt.Errorf("failed to get variable VARIABLE_ONE: HTTP 400 (https://api.github.com/repos/owner/repo/actions/variables/VARIABLE_ONE)"),
		},
	}

	for _, tt := range tests {
		var runTest = func(tty bool) func(t *testing.T) {
			return func(t *testing.T) {
				reg := &httpmock.Registry{}
				tt.httpStubs(reg)
				defer reg.Verify(t)

				ios, _, stdout, _ := iostreams.Test()
				ios.SetStdoutTTY(tty)

				tt.opts.IO = ios
				tt.opts.BaseRepo = func() (ghrepo.Interface, error) {
					return ghrepo.FromFullName("owner/repo")
				}
				tt.opts.HttpClient = func() (*http.Client, error) {
					return &http.Client{Transport: reg}, nil
				}
				tt.opts.Config = func() (gh.Config, error) {
					return config.NewBlankConfig(), nil
				}

				err := getRun(tt.opts)
				if err != nil {
					require.EqualError(t, tt.wantErr, err.Error())
					return
				}

				require.NoError(t, err)
				require.Equal(t, tt.wantOut, stdout.String())
			}
		}

		t.Run(tt.name+" tty", runTest(true))
		t.Run(tt.name+" no-tty", runTest(false))
	}
}
