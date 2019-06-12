package main

import (
	"flag"
	"fmt"
	"k8s.io/klog"
	"os"
	"time"

	"github.com/spf13/pflag"
	apiv1 "k8s.io/api/core/v1"

	"k8s.io/k8s-deploy-operator/internal/net"
)


const (
	defaultLeaderElection          = true
	defaultLeaderElectionID        = "k8s-canary-deploy-controller-leader"
	defaultLeaderElectionNamespace = ""
	defaultWatchNamespace          = apiv1.NamespaceAll
	defaultSyncPeriod              = 60 * time.Minute
	defaultHealthCheckPeriod       = 1 * time.Minute
	defaultHealthzPort             = 10254
	defaultProfilingEnabled        = true
)


// Options defines the commandline interface of this binary
type Options struct {
	ShowVersion bool

	APIServerHost  string
	KubeConfigFile string

	LeaderElection          bool
	LeaderElectionID        string
	LeaderElectionNamespace string

	WatchNamespace    string
	SyncPeriod        time.Duration
	HealthCheckPeriod time.Duration
	HealthzPort       int
	ProfilingEnabled  bool
}

func (options *Options) BindFlags(fs *pflag.FlagSet) {
	fs.BoolVar(&options.ShowVersion, "version", false,
		`Show release information about the k8s canary deploy controller and exit.`)
	fs.StringVar(&options.APIServerHost, "apiserver-host", "",
		`Address of the Kubernetes API server.
		Takes the form "protocol://address:port". If not specified, it is assumed the
		program runs inside a Kubernetes cluster and local discovery is attempted.`)
	fs.StringVar(&options.KubeConfigFile, "kubeconfig", "",
		`Path to a kubeconfig file containing authorization and API server information.`)
	fs.BoolVar(&options.LeaderElection, "election", defaultLeaderElection,
		`Whether we do leader election for ingress controller`)
	fs.StringVar(&options.LeaderElectionID, "election-id", defaultLeaderElectionID,
		`Namespace of leader-election configmap for ingress controller`)
	fs.StringVar(&options.LeaderElectionNamespace, "election-namespace", defaultLeaderElectionNamespace,
		`Namespace of leader-election configmap for ingress controller. If unspecified, the namespace of this controller pod will be used`)
	fs.StringVar(&options.WatchNamespace, "watch-namespace", defaultWatchNamespace,
		`Namespace the controller watches for updates to Kubernetes objects.
		This includes Ingresses, Services and all configuration resources. All
		namespaces are watched if this parameter is left empty.`)
	fs.DurationVar(&options.SyncPeriod, "sync-period", defaultSyncPeriod,
		`Period at which the controller forces the repopulation of its local object stores.`)
	fs.DurationVar(&options.HealthCheckPeriod, "health-check-period", defaultHealthCheckPeriod,
		`Period at which the controller executes AWS health checks for its healthz endpoint.`)
	fs.IntVar(&options.HealthzPort, "healthz-port", defaultHealthzPort,
		`Port to use for the healthz endpoint.`)
	fs.BoolVar(&options.ProfilingEnabled, "profiling", defaultProfilingEnabled,
		`Enable profiling via web interface host:port/debug/pprof/`)
}

func (options *Options) Validate() error {
	if !net.IsPortAvailable(options.HealthzPort) {
		return fmt.Errorf("port %v is already in use. Please check the flag --healthz-port", options.HealthzPort)
	}
	return nil
}

func getOptions() (*Options, error) {
	options := &Options{}

	fs := pflag.NewFlagSet("", pflag.ExitOnError)
	options.BindFlags(fs)

	_ = flag.Set("logtostderr", "true")
	fs.AddGoFlagSet(flag.CommandLine)

	klogFs := flag.NewFlagSet("klog", flag.ExitOnError)
	klog.InitFlags(klogFs)
	fs.AddGoFlagSet(klogFs)

	_ = fs.Parse(os.Args)
	if err := options.Validate(); err != nil {
		return nil, err
	}

	return options, nil
}

