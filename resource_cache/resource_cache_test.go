package resource_cache

import (
	"context"
	"os"
	"testing"

	"go.uber.org/zap"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	ctrlzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var k8sClient client.Client
var logger *zap.Logger
var testEnv *envtest.Environment

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
}

func Run(enableLeaderElection bool, config *rest.Config, signalHandler context.Context) {

	_, err := ctrl.NewManager(config, ctrl.Options{
		Scheme:           scheme,
		Port:             9443,
		LeaderElection:   enableLeaderElection,
		LeaderElectionID: "068b0003.cloud.redhat.com",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}
}

func TestMain(m *testing.M) {
	// call flag.Parse() here if TestMain uses flags
	ctrl.SetLogger(ctrlzap.New(ctrlzap.UseDevMode(true)))
	logger, _ = zap.NewProduction()
	defer logger.Sync()
	logger.Info("bootstrapping test environment")

	testEnv = &envtest.Environment{}

	cfg, err := testEnv.Start()

	if err != nil {
		logger.Fatal("Error starting test env", zap.Error(err))
	}

	if cfg == nil {
		logger.Fatal("env config was returned nil")
	}

	// +kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: clientgoscheme.Scheme})

	if err != nil {
		logger.Fatal("Failed to create k8s client", zap.Error(err))
	}

	if k8sClient == nil {
		logger.Fatal("k8sClient was returned nil", zap.Error(err))
	}

	ctx := context.Background()

	nsSpec := &core.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kafka"}}
	k8sClient.Create(ctx, nsSpec)

	stopManager, cancel := context.WithCancel(context.Background())
	go Run(false, testEnv.Config, stopManager)

	retCode := m.Run()
	logger.Info("Stopping test env...")
	cancel()
	err = testEnv.Stop()

	if err != nil {
		logger.Fatal("Failed to tear down env", zap.Error(err))
	}
	os.Exit(retCode)
}

func TestObjectCache(t *testing.T) {

	config := CacheConfig{
		scheme:        scheme,
		possibleGVKs:  make(map[schema.GroupVersionKind]bool),
		protectedGVKs: make(map[schema.GroupVersionKind]bool),
	}
	oCache := NewObjectCache(context.Background(), k8sClient, &config)

	nn := types.NamespacedName{
		Name:      "test-service",
		Namespace: "default",
	}

	s := core.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nn.Name,
			Namespace: nn.Namespace,
		},
		Spec: core.ServiceSpec{
			Ports: []core.ServicePort{{
				Name: "port-01",
				Port: 1234,
			}},
		},
	}

	SingleIdent := ResourceIdentSingle{
		Provider: "TEST",
		Purpose:  "MAIN",
		Type:     &core.Service{},
	}

	err := oCache.Create(SingleIdent, nn, &s)
	if err != nil {
		t.Error(err)
		return
	}

	obtainedService := core.Service{}

	err = oCache.Get(SingleIdent, &obtainedService)
	if err != nil {
		t.Error(err)
		return
	}

	if obtainedService.Spec.Ports[0].Port != 1234 {
		t.Errorf("Obtained service did not have port 1234")
		return
	}

	obtainedService.Spec.Ports[0].Port = 2345

	err = oCache.Update(SingleIdent, &obtainedService)
	if err != nil {
		t.Error(err)
		return
	}

	updatedService := core.Service{}

	err = oCache.Get(SingleIdent, &updatedService)
	if err != nil {
		t.Error(err)
		return
	}

	if updatedService.Spec.Ports[0].Port != 2345 {
		t.Errorf("Updated service port was not updated")
		return
	}

	MultiIdent := NewMultiResourceIdent("TEST", "MULTI", &core.Service{})

	sm := core.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nn.Name + "-multi",
			Namespace: nn.Namespace,
		},
		Spec: core.ServiceSpec{
			Ports: []core.ServicePort{{
				Name: "port-01",
				Port: 5432,
			}},
		},
	}

	err = oCache.Create(MultiIdent, nn, &sm)
	if err != nil {
		t.Error(err)
		return
	}

	sList := core.ServiceList{}
	err = oCache.List(MultiIdent, &sList)

	if err != nil {
		t.Error(err)
		return
	}

	for _, i := range sList.Items {
		if i.Spec.Ports[0].Port != 5432 {
			t.Errorf("Item not found in list")
			return
		}
	}

	err = oCache.ApplyAll()

	if err != nil {
		t.Error(err)
		return
	}

	clientService := core.Service{}
	if err = k8sClient.Get(context.Background(), types.NamespacedName{
		Namespace: "default",
		Name:      "test-service",
	}, &clientService); err != nil {
		t.Error(err)
		return
	}

	if clientService.Spec.Ports[0].Port != 2345 {
		t.Errorf("Retrieved object has wrong port")
		return
	}

	clientServiceMulti := core.Service{}
	if err = k8sClient.Get(context.Background(), types.NamespacedName{
		Namespace: "default",
		Name:      "test-service-multi",
	}, &clientServiceMulti); err != nil {
		t.Error(err)
		return
	}

	if clientServiceMulti.Spec.Ports[0].Port != 5432 {
		t.Errorf("Retrieved object has wrong port")
		return
	}

	TemplateIdent := NewSingleResourceIdent("TEST", "TEMPLATE", &core.Pod{})

	tnn := types.NamespacedName{
		Name:      "template",
		Namespace: "template-namespace",
	}
	service := &core.Service{}

	if err := oCache.Create(TemplateIdent, tnn, service); err == nil {
		t.Fatal(err)
		t.Fatal("Did not error when should have: cache create")
	}

	if err := oCache.Update(TemplateIdent, service); err == nil {
		t.Fatal("Did not error when should have: cache update")
	}
}
