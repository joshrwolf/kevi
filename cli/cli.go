package cli

import (
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	"cattle.io/kevi/api/v1alpha1"
	"cattle.io/kevi/cli/version"
)

var (
	scheme = runtime.NewScheme()
)

const (
	defaultName      = "kevi"
	defaultNamespace = "kevi-system"
	mwhCertsName     = "kevi-webhook-server-cert"
	mwhName          = "kevi-mutating-webhook-configuration"
	ownerKey         = "kevi.cattle.io"
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func Cli() *cobra.Command {
	cmd := &cobra.Command{
		Use: "kevi",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	addManager(cmd)
	addPack(cmd)
	addCopy(cmd)
	addDeploy(cmd)
	version.AddVersion(cmd)

	return cmd
}

func plog() zerolog.Logger {
	output := zerolog.ConsoleWriter{Out: os.Stdout}
	return log.Output(output)
}
