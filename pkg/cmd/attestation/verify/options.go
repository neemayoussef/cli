package verify

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cli/cli/v2/internal/gh"
	"github.com/cli/cli/v2/pkg/cmd/attestation/api"
	"github.com/cli/cli/v2/pkg/cmd/attestation/artifact/oci"
	"github.com/cli/cli/v2/pkg/cmd/attestation/io"
	"github.com/cli/cli/v2/pkg/cmd/attestation/verification"
	"github.com/cli/cli/v2/pkg/cmdutil"
)

// Options captures the options for the verify command
type Options struct {
	ArtifactPath         string
	BundlePath           string
	Config               func() (gh.Config, error)
	CustomTrustedRoot    string
	DenySelfHostedRunner bool
	DigestAlgorithm      string
	Limit                int
	NoPublicGood         bool
	OIDCIssuer           string
	Owner                string
	PredicateType        string
	Repo                 string
	SAN                  string
	SANRegex             string
	SignerRepo           string
	SignerWorkflow       string
	APIClient            api.Client
	Logger               *io.Handler
	OCIClient            oci.Client
	SigstoreVerifier     verification.SigstoreVerifier
	exporter             cmdutil.Exporter
}

// Clean cleans the file path option values
func (opts *Options) Clean() {
	if opts.BundlePath != "" {
		opts.BundlePath = filepath.Clean(opts.BundlePath)
	}
}

func (opts *Options) SetPolicyFlags() {
	// check that Repo is in the expected format if provided
	if opts.Repo != "" {
		// we expect the repo argument to be in the format <OWNER>/<REPO>
		splitRepo := strings.Split(opts.Repo, "/")

		// if Repo is provided but owner is not, set the OWNER portion of the Repo value
		// to Owner
		opts.Owner = splitRepo[0]

		if !isSignerIdentityProvided(opts) {
			opts.SANRegex = expandToGitHubURL(opts.Repo)
		}
		return
	}
	if !isSignerIdentityProvided(opts) {
		opts.SANRegex = expandToGitHubURL(opts.Owner)
	}
}

// AreFlagsValid checks that the provided flag combination is valid
// and returns an error otherwise
func (opts *Options) AreFlagsValid() error {
	// If provided, check that the Repo option is in the expected format <OWNER>/<REPO>
	if opts.Repo != "" && !isProvidedRepoValid(opts.Repo) {
		return fmt.Errorf("invalid value provided for repo: %s", opts.Repo)
	}

	// If provided, check that the SignerRepo option is in the expected format <OWNER>/<REPO>
	if opts.SignerRepo != "" && !isProvidedRepoValid(opts.SignerRepo) {
		return fmt.Errorf("invalid value provided for signer-repo: %s", opts.SignerRepo)
	}

	// Check that limit is between 1 and 1000
	if opts.Limit < 1 || opts.Limit > 1000 {
		return fmt.Errorf("limit %d not allowed, must be between 1 and 1000", opts.Limit)
	}

	return nil
}

// check if any of the signer identity flags have been provided
func isSignerIdentityProvided(opts *Options) bool {
	return opts.SAN != "" || opts.SANRegex != "" || opts.SignerRepo != "" || opts.SignerWorkflow != ""
}

func isProvidedRepoValid(repo string) bool {
	// we expect a provided repository argument be in the format <OWNER>/<REPO>
	splitRepo := strings.Split(repo, "/")
	return len(splitRepo) == 2
}
