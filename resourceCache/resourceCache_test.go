package resourcecache

import (
	"context"
	"os"
	"sort"
	"strconv"
	"testing"
	"time"

	"github.com/RedHatInsights/rhc-osdk-utils/utils"
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
	log      logr.Logger
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(apps.AddToScheme(scheme))
}

func Run(_ context.Context, enableLeaderElection bool, config *rest.Config) {

	_, err := ctrl.NewManager(config, ctrl.Options{
		Scheme:           scheme,
		LeaderElection:   enableLeaderElection,
		LeaderElectionID: "068b0003.cloud.redhat.com",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}
}

func syncLogger(logger *zap.Logger) {
	_ = logger.Sync()
}

func TestMain(m *testing.M) {
	// call flag.Parse() here if TestMain uses flags
	ctrl.SetLogger(ctrlzap.New(ctrlzap.UseDevMode(true)))
	logger, _ = zap.NewProduction()
	log = zapr.NewLogger(logger)
	defer syncLogger(logger)
	log.Info("bootstrapping test environment")

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
	err = k8sClient.Create(ctx, nsSpec)
	if err != nil {
		logger.Fatal("could not create namespace", zap.Error(err))
	}

	stopManager, cancel := context.WithCancel(context.Background())
	go Run(stopManager, false, testEnv.Config)

	m.Run()
	logger.Info("Stopping test env...")
	cancel()
	err = testEnv.Stop()

	if err != nil {
		logger.Fatal("Failed to tear down env", zap.Error(err))
	}
}

type Key string

func TestObjectCache(t *testing.T) {

	config := NewCacheConfig(scheme, nil, nil)

	ctx := context.Background()
	zapLog, _ := zap.NewDevelopment()

	log := zapr.NewLogger(zapLog)

	oCache := NewObjectCache(ctx, k8sClient, &log, config)

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

	config := NewCacheConfig(scheme, nil, nil)

	ctx := context.Background()
	oCache := NewObjectCache(ctx, k8sClient, &log, config)

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

var applyOrder = []string{
	"*",
	"Service",
	"Secret",
	"Deployment",
	"Job",
	"CronJob",
	"ScaledObject",
}

func TestOrderingSort(t *testing.T) {
	config := NewCacheConfig(scheme, nil, nil, Options{
		Ordering: applyOrder,
	})

	ctx := context.Background()
	oCache := NewObjectCache(ctx, k8sClient, &log, config)

	nn := types.NamespacedName{
		Name:      "test-ordering",
		Namespace: "default",
	}

	cf := core.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nn.Name,
			Namespace: nn.Namespace,
		},
		Data: map[string]string{
			"happy":     "good",
			"colour me": "good",
		},
	}

	SingleIdentCF := ResourceIdentSingle{
		Provider: "TEST",
		Purpose:  "MAIN-CF",
		Type:     &core.ConfigMap{},
	}

	err := oCache.Create(SingleIdentCF, nn, &cf)
	assert.Nil(t, err, "error from create call for cf")

	sec := core.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nn.Name,
			Namespace: nn.Namespace,
		},
		StringData: map[string]string{
			"happy":     "good",
			"colour me": "good",
		},
	}

	SingleIdentSec := ResourceIdentSingle{
		Provider: "TEST",
		Purpose:  "MAIN-SEC",
		Type:     &core.Secret{},
	}

	err = oCache.Create(SingleIdentSec, nn, &sec)
	assert.Nil(t, err, "error from create call for sec")

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

	SingleIdentDeploy := ResourceIdentSingle{
		Provider: "TEST",
		Purpose:  "MAIN-DEP",
		Type:     &apps.Deployment{},
	}

	err = oCache.Create(SingleIdentDeploy, nn, &a)
	assert.Nil(t, err, "error from create call")

	ss := createRandomServices(250)
	for _, s := range ss {
		err = oCache.Create(s.ident, s.nn, s.obj)
		assert.Nil(t, err, "error from create call inside 250 loop")
	}

	dataToSort := objectsToApply{scheme: oCache.scheme, order: applyOrder}
	for res := range oCache.data {
		for nn := range oCache.data[res] {
			dataToSort.objs = append(dataToSort.objs, ObjectToApply{
				Ident:          res,
				NamespacedName: nn,
				Resource:       oCache.data[res][nn],
			})
		}
	}

	sort.Sort(dataToSort)
	gvk, err := utils.GetKindFromObj(oCache.scheme, dataToSort.objs[len(dataToSort.objs)-1].Ident.GetType())
	assert.NoError(t, err)
	assert.Equal(t, "Deployment", gvk.Kind)
	gvk, err = utils.GetKindFromObj(oCache.scheme, dataToSort.objs[len(dataToSort.objs)-2].Ident.GetType())
	assert.NoError(t, err)
	assert.Equal(t, "Secret", gvk.Kind)
}

func TestObjectCachePreseedStrictFail(t *testing.T) {

	config := NewCacheConfig(scheme, nil, nil, Options{
		StrictGVK:    true,
		DebugOptions: DebugOptions{},
	},
	)

	ctx := context.Background()

	oCache := NewObjectCache(ctx, k8sClient, &log, config)

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

	config := NewCacheConfig(
		scheme,
		GVKMap{
			{
				Group:   "apps",
				Version: "v1",
				Kind:    "Deployment",
			}: true,
		},
		nil,
		Options{
			StrictGVK: true,
		},
	)

	ctx := context.Background()

	oCache := NewObjectCache(ctx, k8sClient, &log, config)

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

func TestObjectCacheNonMatchingObject(t *testing.T) {

	ctx := context.Background()
	config := NewCacheConfig(scheme, nil, nil)
	oCache := NewObjectCache(ctx, k8sClient, &log, config)

	nn := types.NamespacedName{
		Name:      "test-writenow",
		Namespace: "default",
	}

	a := core.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nn.Name,
			Namespace: nn.Namespace,
		},
	}

	SingleIdent := ResourceIdentSingle{
		Provider: "TEST",
		Purpose:  "MAIN",
		Type:     &core.Service{},
		WriteNow: true,
	}

	err := oCache.Create(SingleIdent, nn, &a)
	assert.ErrorContains(t, err, "create: resourceIdent type does not match runtime object [default/test-writenow] [/v1, Kind=Service] [/v1, Kind=ConfigMap]", err)
}

func TestObjectCacheWriteNow(t *testing.T) {

	ctx := context.Background()
	config := NewCacheConfig(scheme, nil, nil)
	oCache := NewObjectCache(ctx, k8sClient, &log, config)

	nn := types.NamespacedName{
		Name:      "test-writenow",
		Namespace: "default",
	}

	a := core.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nn.Name,
			Namespace: nn.Namespace,
		},
	}

	SingleIdent := ResourceIdentSingle{
		Provider: "TEST",
		Purpose:  "MAIN",
		Type:     &core.ConfigMap{},
		WriteNow: true,
	}

	err := oCache.Create(SingleIdent, nn, &a)
	assert.Nil(t, err, "create error wasn't nil")

	err = oCache.Update(SingleIdent, &a)
	assert.Nil(t, err, "create error wasn't nil")

	cfgmap := core.ConfigMap{}

	err = k8sClient.Get(context.Background(), nn, &cfgmap)
	assert.Nil(t, err, "error from k8s client get for configmap")
}

func TestObjectReconcile(t *testing.T) {

	config := NewCacheConfig(scheme, nil, nil)

	ctx := context.Background()

	oCache := NewObjectCache(ctx, k8sClient, &log, config)

	nn := types.NamespacedName{
		Name:      "test-reconcile",
		Namespace: "default",
	}

	owner := core.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nn.Name + "owner",
			Namespace: nn.Namespace,
		},
	}

	err := k8sClient.Create(ctx, &owner)
	assert.Nil(t, err, "create error wasn't nil")
	err = k8sClient.Get(ctx, types.NamespacedName{
		Name:      nn.Name + "owner",
		Namespace: nn.Namespace,
	}, &owner)
	assert.Nil(t, err, "get error wasn't nil")

	a := core.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nn.Name,
			Namespace: nn.Namespace,
		},
	}

	SingleIdent := ResourceIdentSingle{
		Provider: "TEST",
		Purpose:  "MAIN",
		Type:     &core.ConfigMap{},
		WriteNow: true,
	}

	err = oCache.Create(SingleIdent, nn, &a)
	assert.Nil(t, err, "create error wasn't nil")

	a.ObjectMeta.OwnerReferences = []metav1.OwnerReference{{
		APIVersion: "v1",
		Kind:       "ConfigMap",
		Name:       nn.Name + "owner",
		UID:        owner.UID,
	}}

	err = oCache.Update(SingleIdent, &a)
	assert.Nil(t, err, "create error wasn't nil")

	err = oCache.ApplyAll()
	assert.NoError(t, err, "apply error wasn't nil")

	err = k8sClient.Get(context.Background(), nn, &a)
	assert.Nil(t, err, "error from k8s client get for configmap")

	config2 := NewCacheConfig(scheme, GVKMap{schema.GroupVersionKind{Kind: "ConfigMap", Version: "v1"}: true}, nil)

	oCache2 := NewObjectCache(ctx, k8sClient, &log, config2)
	err = oCache2.Reconcile(owner.UID)
	assert.Nil(t, err, "reconcile error wasn't nil")

	err = k8sClient.Get(context.Background(), nn, &a)
	assert.NotNil(t, err, "shouldn't be able to get object")
}

func TestObjectReconcileProtected(t *testing.T) {
	protectedConfigMapGVK, _ := utils.GetKindFromObj(scheme, &core.ConfigMap{})
	config := NewCacheConfig(scheme, nil, nil)

	ctx := context.Background()

	oCache := NewObjectCache(ctx, k8sClient, &log, config)

	nn := types.NamespacedName{
		Name:      "test-reconcile-protected",
		Namespace: "default",
	}

	owner := core.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nn.Name + "owner",
			Namespace: nn.Namespace,
		},
	}

	err := k8sClient.Create(ctx, &owner)
	assert.Nil(t, err, "create error wasn't nil")
	err = k8sClient.Get(ctx, types.NamespacedName{
		Name:      nn.Name + "owner",
		Namespace: nn.Namespace,
	}, &owner)
	assert.Nil(t, err, "get error wasn't nil")

	a := core.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nn.Name,
			Namespace: nn.Namespace,
		},
	}

	b := core.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nn.Name,
			Namespace: nn.Namespace,
		},
	}

	SingleIdent := ResourceIdentSingle{
		Provider: "TEST",
		Purpose:  "MAIN",
		Type:     &core.ConfigMap{},
	}

	SingleIdentSecret := ResourceIdentSingle{
		Provider: "TEST",
		Purpose:  "MAINSec",
		Type:     &core.Secret{},
	}

	err = oCache.Create(SingleIdent, nn, &a)
	assert.Nil(t, err, "create error wasn't nil")

	err = oCache.Create(SingleIdentSecret, nn, &b)
	assert.Nil(t, err, "create error wasn't nil")

	a.ObjectMeta.OwnerReferences = []metav1.OwnerReference{{
		APIVersion: "v1",
		Kind:       "ConfigMap",
		Name:       nn.Name + "owner",
		UID:        owner.UID,
	}}

	b.ObjectMeta.OwnerReferences = []metav1.OwnerReference{{
		APIVersion: "v1",
		Kind:       "ConfigMap",
		Name:       nn.Name + "owner",
		UID:        owner.UID,
	}}

	err = oCache.Update(SingleIdent, &a)
	assert.Nil(t, err, "create error wasn't nil")

	err = oCache.Update(SingleIdentSecret, &b)
	assert.Nil(t, err, "create error wasn't nil")

	oCache.Debug()

	err = oCache.ApplyAll()
	assert.Nil(t, err, "apply error wasn't nil")

	err = k8sClient.Get(context.Background(), nn, &a)
	assert.Nil(t, err, "error from k8s client get for configmap")

	err = k8sClient.Get(context.Background(), nn, &b)
	assert.Nil(t, err, "error from k8s client get for secret")

	config2 := NewCacheConfig(scheme,
		nil,
		GVKMap{
			protectedConfigMapGVK: true,
		})

	oCache2 := NewObjectCache(ctx, k8sClient, &log, config2)

	oCache2.AddPossibleGVKFromIdent(SingleIdent)
	oCache2.AddPossibleGVKFromIdent(SingleIdentSecret)

	err = oCache2.ApplyAll()
	assert.Nil(t, err, "apply error wasn't nil")

	oCache.Debug()

	err = oCache2.Reconcile(owner.UID)
	assert.Nil(t, err, "reconcile error wasn't nil")

	assert.Eventually(t, func() bool {
		err := k8sClient.Get(context.Background(), nn, &b)
		return err != nil
	},
		10*time.Second, 250*time.Millisecond,
		"non-protected resource never disappeared",
	)
	time.Sleep(2000)
	err = k8sClient.Get(context.Background(), nn, &a)
	assert.Nil(t, err, "couldn't get the protected resource")

}

func TestCacheAddPossibleGVK(t *testing.T) {

	config := NewCacheConfig(scheme, nil, nil)
	ctx := context.Background()
	oCache := NewObjectCache(ctx, k8sClient, &log, config)

	SingleIdent := ResourceIdentSingle{
		Provider: "TEST",
		Purpose:  "MAIN",
		Type:     &core.ConfigMap{},
		WriteNow: true,
	}

	oCache.AddPossibleGVKFromIdent(SingleIdent)

	obj, err := utils.GetKindFromObj(scheme, SingleIdent.GetType())
	assert.Nil(t, err, "get object was not nil")
	assert.Contains(t, oCache.config.possibleGVKs, obj)
}
