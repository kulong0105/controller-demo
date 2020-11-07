package main

import (
	"flag"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"k8s.io/klog"

	informers "github.com/kulong0105/controller-demo/pkg/client/informers/externalversions"

	clientset "github.com/kulong0105/controller-demo/pkg/client/clientset/versioned"
)

var (
	onlyOneSignalHandler = make(chan struct{})
	shutdownSignals      = []os.Signal{syscall.SIGINT, syscall.SIGTERM}
)

func setupSignalHandler() (stopCh <-chan struct{}) {
	close(onlyOneSignalHandler)

	stop := make(chan struct{})
	c := make(chan os.Signal, 2)

	signal.Notify(c, shutdownSignals...)

	go func() {
		<-c
		close(stop)
		<-c
		os.Exit(1)

	}()

	return stop
}

func initClient() (*kubernetes.Clientset, *rest.Config, error) {

	var err error
	var config *rest.Config

	var kubeconfig *string

	if home := homedir.HomeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "optinal path")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path")
	}

	flag.Parse()

	if config, err = rest.InClusterConfig(); err != nil {
		if config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig); err != nil {
			panic(err.Error())
		}
	}

	kubeclient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, config, err
	}

	return kubeclient, config, nil
}

func main() {
	flag.Parse()

	stopCh := setupSignalHandler()

	_, cfg, err := initClient()
	if err != nil {
		klog.Fatalf("Error building k8s clientset: %s", err.Error())
	}

	crontabClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		klog.Fatalf("Error build crontab clientset: %s", err.Error())
	}

	crontabInfomerFactory := informers.NewSharedInformerFactory(crontabClient, time.Second*30)

	controller := NewController(crontabInfomerFactory.Stable().V1beta1().CronTabs())

	go crontabInfomerFactory.Start(stopCh)

	if err := controller.Run(2, stopCh); err != nil {
		klog.Fatalf("Error running controller: %s", err.Error())
	}
}
