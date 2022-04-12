package status

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/RedHatInsights/clowder/controllers/cloud.redhat.com/errors"
	apps "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	statusTypes "github.com/RedHatInsights/rhc-osdk-utils/status/types"
)

//Count deployments for a StatusSource
func countDeployments(ctx context.Context, pClient client.Client, statusSource statusTypes.StatusSource, namespaces []string) (int32, int32, string, error) {
	var managedDeployments int32
	var readyDeployments int32
	var brokenDeployments []string
	var msg = ""

	deployments := []apps.Deployment{}
	for _, namespace := range namespaces {
		opts := []client.ListOption{
			client.InNamespace(namespace),
		}
		tmpDeployments := apps.DeploymentList{}
		err := pClient.List(ctx, &tmpDeployments, opts...)
		if err != nil {
			return 0, 0, "", err
		}
		deployments = append(deployments, tmpDeployments.Items...)
	}

	// filter for resources owned by the ClowdObject and check their status
	for _, deployment := range deployments {
		for _, owner := range deployment.GetOwnerReferences() {
			if owner.UID == statusSource.GetUID() {
				managedDeployments++
				if ok := deploymentStatusChecker(deployment); ok {
					readyDeployments++
				} else {
					brokenDeployments = append(brokenDeployments, fmt.Sprintf("%s/%s", deployment.Name, deployment.Namespace))
				}
				break
			}
		}
	}

	if len(brokenDeployments) > 0 {
		sort.Strings(brokenDeployments)
		msg = fmt.Sprintf("broken deployments: [%s]", strings.Join(brokenDeployments, ", "))
	}

	return managedDeployments, readyDeployments, msg, nil
}

//Checks the status for a given deployment to ensure it is Available and True
func deploymentStatusChecker(deployment apps.Deployment) bool {
	if deployment.Generation > deployment.Status.ObservedGeneration {
		// The status on this resource needs to update
		return false
	}

	for _, condition := range deployment.Status.Conditions {
		if condition.Type == "Available" && condition.Status == "True" {
			return true
		}
	}

	return false
}

//Gets resource figures for a given StatusSource. Allows for custom figures via the GetObjectSpecificFigures call
func GetResourceFigures(ctx context.Context, client client.Client, statusSource statusTypes.StatusSource) (statusTypes.StatusSourceFigures, string, error) {
	figures := statusTypes.StatusSourceFigures{}
	msg := ""
	namespaces, err := statusSource.GetNamespaces(ctx, client)
	if err != nil {
		return figures, "", errors.Wrap("get namespaces: ", err)
	}

	managedDeployments, readyDeployments, _, err := countDeployments(ctx, client, statusSource, namespaces)
	if err != nil {
		return figures, "", errors.Wrap("count deploys: ", err)
	}

	figures.ManagedDeployments += managedDeployments
	figures.ReadyDeployments += readyDeployments

	specialFigures, msg, err := statusSource.GetObjectSpecificFigures(ctx, client)
	if err != nil {
		return figures, msg, err
	}
	figures = statusSource.AddDeploymentFigures(figures, specialFigures)

	return figures, msg, nil
}

//Determines if all deployments are ready based on all of the resource figures for a StatusSource
func GetResourceStatus(ctx context.Context, client client.Client, statusSource statusTypes.StatusSource) (bool, string, error) {
	stats, msg, err := GetResourceFigures(ctx, client, statusSource)
	if err != nil {
		return false, msg, err
	}
	return statusSource.AreDeploymentsReady(stats), msg, nil
}
