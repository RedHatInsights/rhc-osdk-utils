package status

import (
	"context"
	"testing"

	cond "sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"

	statusTypes "github.com/RedHatInsights/rhc-osdk-utils/status/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

type Client struct {
	mock.Mock
}
type RESTMapper struct {
	meta.RESTMapper
}
type StatusWriter struct {
	client.StatusWriter
}

func (c *Client) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	args := c.Called(ctx, obj, opts)
	return args.Error(0)
}
func (c *Client) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	args := c.Called(ctx, obj, opts)
	return args.Error(0)
}
func (c *Client) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	args := c.Called(ctx, obj, opts)
	return args.Error(0)
}
func (c *Client) DeleteAllOf(ctx context.Context, obj client.Object, opts ...client.DeleteAllOfOption) error {
	args := c.Called(ctx, obj, opts)
	return args.Error(0)
}
func (c *Client) Get(ctx context.Context, key types.NamespacedName, obj client.Object) error {
	args := c.Called(ctx, key, obj)
	return args.Error(0)
}
func (c *Client) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	c.Called()
	return nil
}
func (c *Client) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	args := c.Called(ctx, obj, patch, opts)
	return args.Error(0)
}
func (c *Client) RESTMapper() meta.RESTMapper {
	c.Called()
	return RESTMapper{}
}
func (c *Client) Scheme() *runtime.Scheme {
	c.Called()
	s := runtime.Scheme{}
	return &s
}
func (c *Client) Status() client.StatusWriter {
	c.Called()
	s := StatusWriter{}
	return s
}

type StatusSourceMock struct {
	status             bool
	ManagedDeployments int32
	ReadyDeployments   int32
	cond.Setter
}

func (i *StatusSourceMock) SetStatusReady(ready bool) {
	i.status = ready
}
func (i *StatusSourceMock) GetNamespaces(ctx context.Context, pClient client.Client) ([]string, error) {
	return []string{"testa", "testb"}, nil
}
func (i *StatusSourceMock) SetDeploymentFigures(figures statusTypes.StatusSourceFigures) {
	i.ManagedDeployments = figures.ManagedDeployments
	i.ReadyDeployments = figures.ReadyDeployments
}
func (i *StatusSourceMock) AreDeploymentsReady(figures statusTypes.StatusSourceFigures) bool {
	return figures.ManagedDeployments == figures.ReadyDeployments
}
func (i *StatusSourceMock) GetObjectSpecificFigures(context.Context, client.Client) (statusTypes.StatusSourceFigures, string, error) {
	return statusTypes.StatusSourceFigures{}, "", nil
}
func (i *StatusSourceMock) AddDeploymentFigures(figsA statusTypes.StatusSourceFigures, figsB statusTypes.StatusSourceFigures) statusTypes.StatusSourceFigures {
	figsA.ManagedDeployments += figsB.ManagedDeployments
	figsA.ReadyDeployments += figsB.ReadyDeployments
	return figsA
}

func Prereqs() (*Client, context.Context, StatusSourceMock) {
	mock := &Client{}
	mock.On("List").Return(nil)
	ctx := context.Background()
	ss := StatusSourceMock{}
	return mock, ctx, ss
}

func TestGetResourceFigures(t *testing.T) {
	mock, ctx, ss := Prereqs()

	figures, message, err := GetResourceFigures(ctx, mock, &ss)

	assert.EqualValues(t, figures.ManagedDeployments, 0)
	assert.EqualValues(t, figures.ManagedTopics, 0)
	assert.EqualValues(t, figures.ReadyDeployments, 0)
	assert.EqualValues(t, figures.ReadyTopics, 0)
	assert.Equal(t, message, "")
	assert.Equal(t, err, nil)
}

func TestGetResourceStatus(t *testing.T) {
	mock, ctx, ss := Prereqs()

	ready, message, err := GetResourceStatus(ctx, mock, &ss)

	assert.Equal(t, ready, true)
	assert.Equal(t, message, "")
	assert.Equal(t, err, nil)
}
