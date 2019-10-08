package main

import (
	"flag"
	log "github.com/sirupsen/logrus"
	"net/http"
	"time"

	"github.com/seniorfoffo/kubernetes-replicator/liveness"
	"github.com/seniorfoffo/kubernetes-replicator/replicate"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var f flags

func init() {
	var err error
	flag.StringVar(&f.Kubeconfig, "kubeconfig", "", "path to Kubernetes config file")
	flag.StringVar(&f.ResyncPeriodS, "resync-period", "30m", "resynchronization period")
	flag.StringVar(&f.StatusAddr, "status-addr", ":9102", "listen address for status and monitoring server")
	flag.StringVar(&f.LogLevel, "loglevel", "Info", "loglevel")
	flag.BoolVar(&f.AllowAll, "allow-all", false, "allow replication of all secrets (CAUTION: only use when you know what you're doing)")
	flag.Parse()

	f.ResyncPeriod, err = time.ParseDuration(f.ResyncPeriodS)
	if err != nil {
		panic(err)
	}
}

func main() {
	var config *rest.Config
	var err error
	var client kubernetes.Interface
	var loglevels map

	loglevels := {
		"Trace": log.TraceLevel,
		"Debug": log.Debuglevel,
		"Info":  log.InfoLevel,
		"Warn":  log.WarnLevel,
		"Error": log.ErrorLevel,
		"Fatal": log.FatalLevel,
		"Panic": log.PanicLevel,
	}

	log.SetLevel(loglevels[f.LogLevel])

	if f.Kubeconfig == "" {
		log.WithFields(log.Fields{
			"configPath": f.Kubeconfig,
		}).Info("using in-cluster configuration")
		config, err = rest.InClusterConfig()
	} else {
		log.WithFields(log.Fields{
			"configPath": f.Kubeconfig,
		}).Info("using configuration from configuration file")
		config, err = clientcmd.BuildConfigFromFlags("", f.Kubeconfig)
	}

	if err != nil {
		panic(err)
	}

	client = kubernetes.NewForConfigOrDie(config)

	secretRepl := replicate.NewSecretReplicator(client, f.ResyncPeriod, f.AllowAll)
	configMapRepl := replicate.NewConfigMapReplicator(client, f.ResyncPeriod, f.AllowAll)

	go func() {
		secretRepl.Run()
	}()

	go func() {
		configMapRepl.Run()
	}()

	h := liveness.Handler{
		Replicators: []replicate.Replicator{secretRepl, configMapRepl},
	}

	log.WithFields(log.Fields{
		"statusAddr": f.statusAddr,
	}).Info("starting liveness monitor")

	http.Handle("/healthz", &h)
	http.ListenAndServe(f.StatusAddr, nil)
}
