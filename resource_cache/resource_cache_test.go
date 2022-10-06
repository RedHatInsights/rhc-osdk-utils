package resource_cache

import (
	"context"
	"os"
	"strconv"
	"testing"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	apps "k8s.io/api/apps/v1"
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

	"github.com/stretchr/testify/assert"
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
	utilruntime.Must(apps.AddToScheme(scheme))
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

type Key string

func TestObjectCache(t *testing.T) {

	config := CacheConfig{
		scheme:        scheme,
		possibleGVKs:  make(map[schema.GroupVersionKind]bool),
		protectedGVKs: make(map[schema.GroupVersionKind]bool),
		logKey:        Key("bunk"),
	}
	var log logr.Logger

	ctx := context.Background()
	zapLog, _ := zap.NewDevelopment()

	log = zapr.NewLogger(zapLog)

	ctx = context.WithValue(ctx, Key("bunk"), &log)

	oCache := NewObjectCache(ctx, k8sClient, &config)

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
	assert.Nil(t, err, "error from cache create was not nil")
	obtainedService := core.Service{}

	err = oCache.Get(SingleIdent, &obtainedService)
	assert.Nil(t, err, "error from cache get was not nil")

	assert.Equal(t, int32(1234), obtainedService.Spec.Ports[0].Port, "Obtained service did not have port 1234")

	obtainedService.Spec.Ports[0].Port = 2345

	err = oCache.Update(SingleIdent, &obtainedService)
	assert.Nil(t, err, "error from update get was not nil")

	updatedService := core.Service{}

	err = oCache.Get(SingleIdent, &updatedService)
	assert.Nil(t, err, "error from cache get was not nil")

	assert.Equal(t, int32(2345), updatedService.Spec.Ports[0].Port, "Updated service port was not updated")

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
	assert.Nil(t, err, "error from create of multi ident")

	sList := core.ServiceList{}
	err = oCache.List(MultiIdent, &sList)
	assert.Nil(t, err, "error from list of multi ident")
	assert.Equal(t, int32(5432), sList.Items[0].Spec.Ports[0].Port, "Item not found in list")

	err = oCache.ApplyAll()
	assert.Nil(t, err, "cache apply failed")

	clientService := core.Service{}
	err = k8sClient.Get(context.Background(), types.NamespacedName{
		Namespace: "default",
		Name:      "test-service",
	}, &clientService)
	assert.Nil(t, err, "item didn't land in k8s")
	assert.Equal(t, int32(2345), clientService.Spec.Ports[0].Port, "Retrieved object has wrong port")

	clientServiceMulti := core.Service{}
	err = k8sClient.Get(context.Background(), types.NamespacedName{
		Namespace: "default",
		Name:      "test-service-multi",
	}, &clientServiceMulti)
	assert.Nil(t, err, "item didn't land in k8s")
	assert.Equal(t, int32(5432), clientServiceMulti.Spec.Ports[0].Port, "Retrieved object has wrong port")

	TemplateIdent := NewSingleResourceIdent("TEST", "TEMPLATE", &core.Pod{})

	tnn := types.NamespacedName{
		Name:      "template",
		Namespace: "template-namespace",
	}
	service := &core.Service{}

	err = oCache.Create(TemplateIdent, tnn, service)
	assert.NotNil(t, err, "Did not error when should have: cache create")

	err = oCache.Update(TemplateIdent, service)
	assert.NotNil(t, err, "Did not error when should have: cache update")
}

type identAndObject struct {
	obj   client.Object
	ident ResourceIdent
	nn    types.NamespacedName
}

func createRandomServices(n int) []identAndObject {
	listOfObjects := []identAndObject{}

	for i := 0; i <= n; i++ {
		is := strconv.Itoa(i)
		nn := types.NamespacedName{
			Name:      "cr-service-" + is,
			Namespace: "default",
		}

		ident := ResourceIdentSingle{
			Provider: "TEST",
			Purpose:  "MAINSERVICE" + is,
			Type:     &core.Service{},
		}

		s := core.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      nn.Name,
				Namespace: nn.Namespace,
			},
			Spec: core.ServiceSpec{
				Ports: []core.ServicePort{{
					Name: "test",
					Port: 9090,
				}},
			},
		}
		listOfObjects = append(listOfObjects, identAndObject{
			obj:   &s,
			ident: ident,
			nn:    nn,
		})
	}
	return listOfObjects
}

func TestObjectCacheOrdering(t *testing.T) {

	config := CacheConfig{
		scheme:        scheme,
		possibleGVKs:  make(map[schema.GroupVersionKind]bool),
		protectedGVKs: make(map[schema.GroupVersionKind]bool),
		logKey:        Key("bunk"),
	}
	var log logr.Logger

	ctx := context.Background()
	zapLog, _ := zap.NewDevelopment()

	log = zapr.NewLogger(zapLog)

	ctx = context.WithValue(ctx, Key("bunk"), &log)

	oCache := NewObjectCache(ctx, k8sClient, &config)

	nn := types.NamespacedName{
		Name:      "test-ordering",
		Namespace: "default",
	}

	a := apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nn.Name,
			Namespace: nn.Namespace,
		},
		Spec: apps.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"test": "test",
				},
			},
			Template: core.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"test": "test",
					},
				},
				Spec: core.PodSpec{
					Containers: []core.Container{{
						Name:  "test",
						Image: "test",
					}},
				},
			},
		},
	}

	SingleIdent := ResourceIdentSingle{
		Provider: "TEST",
		Purpose:  "MAIN",
		Type:     &apps.Deployment{},
	}

	err := oCache.Create(SingleIdent, nn, &a)
	assert.Nil(t, err, "error from create call")

	ss := createRandomServices(250)
	for _, s := range ss {
		err = oCache.Create(s.ident, s.nn, s.obj)
		assert.Nil(t, err, "error from create call inside 250 loop")
	}

	err = oCache.ApplyAll()
	assert.Nil(t, err, "error from apply cache call")

	deployment := apps.Deployment{}
	err = k8sClient.Get(context.Background(), nn, &deployment)
	assert.Nil(t, err, "error from k8s client get for deployment")

	time := deployment.ObjectMeta.CreationTimestamp.Time

	clientServiceList := core.ServiceList{}
	err = k8sClient.List(context.Background(), &clientServiceList)
	assert.Nil(t, err, "error from list for services call")

	for _, item := range clientServiceList.Items {
		if item.ObjectMeta.CreationTimestamp.Time.After(time) {
			t.Fatal("deployment was created before this resource, error!")
		}
	}
}

func TestObjectCachePreseedStrictFail(t *testing.T) {

	config := CacheConfig{
		scheme:        scheme,
		possibleGVKs:  make(map[schema.GroupVersionKind]bool),
		protectedGVKs: make(map[schema.GroupVersionKind]bool),
		logKey:        Key("bunk"),
		Options: Options{
			StrictGVK: true,
		},
	}
	var log logr.Logger

	ctx := context.Background()
	zapLog, _ := zap.NewDevelopment()

	log = zapr.NewLogger(zapLog)

	ctx = context.WithValue(ctx, Key("bunk"), &log)

	oCache := NewObjectCache(ctx, k8sClient, &config)

	nn := types.NamespacedName{
		Name:      "test-ordering",
		Namespace: "default",
	}

	a := apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nn.Name,
			Namespace: nn.Namespace,
		},
		Spec: apps.DeploymentSpec{},
	}

	SingleIdent := ResourceIdentSingle{
		Provider: "TEST",
		Purpose:  "MAIN",
		Type:     &apps.Deployment{},
	}

	err := oCache.Create(SingleIdent, nn, &a)
	assert.NotNil(t, err, "create error wasn't nil")
}

func TestObjectCachePreseedStrictPass(t *testing.T) {

	config := CacheConfig{
		scheme: scheme,
		possibleGVKs: map[schema.GroupVersionKind]bool{
			{
				Group:   "apps",
				Version: "v1",
				Kind:    "Deployment",
			}: true,
		},
		protectedGVKs: make(map[schema.GroupVersionKind]bool),
		logKey:        Key("bunk"),
		Options: Options{
			StrictGVK: true,
		},
	}
	var log logr.Logger

	ctx := context.Background()
	zapLog, _ := zap.NewDevelopment()

	log = zapr.NewLogger(zapLog)

	ctx = context.WithValue(ctx, Key("bunk"), &log)

	oCache := NewObjectCache(ctx, k8sClient, &config)

	nn := types.NamespacedName{
		Name:      "test-ordering",
		Namespace: "default",
	}

	a := apps.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nn.Name,
			Namespace: nn.Namespace,
		},
		Spec: apps.DeploymentSpec{},
	}

	SingleIdent := ResourceIdentSingle{
		Provider: "TEST",
		Purpose:  "MAIN",
		Type:     &apps.Deployment{},
	}

	err := oCache.Create(SingleIdent, nn, &a)
	assert.Nil(t, err, "create error wasn't nil")
}
