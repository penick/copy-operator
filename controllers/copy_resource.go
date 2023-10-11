package controllers

import (
	"context"
	"reflect"

	v1apps "k8s.io/api/apps/v1"
	v1core "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type CopyResource interface {
	GetList(ctx context.Context, cl client.Client, namespace string) ([]client.Object, error)
	CompareSpec(src, dest client.Object) bool
	CopySpec(src, dest client.Object)
}

type CopyDeployment struct{}

var copyDeploy CopyDeployment

func (c *CopyDeployment) GetList(ctx context.Context, cl client.Client, namespace string) ([]client.Object, error) {
	deploys := v1apps.DeploymentList{}
	err := cl.List(ctx, &deploys, &client.ListOptions{Namespace: namespace})
	if err != nil {
		return nil, err
	}
	objects := make([]client.Object, len(deploys.Items))
	for i, deploy := range deploys.Items {
		objects[i] = &deploy
	}
	return objects, nil
}

func (c *CopyDeployment) CompareSpec(src, dest client.Object) bool {
	return reflect.DeepEqual(src.(*v1apps.Deployment).Spec, dest.(*v1apps.Deployment))
}

func (c *CopyDeployment) CopySpec(src, dest client.Object) {
	dest.(*v1apps.Deployment).Spec = src.(*v1apps.Deployment).Spec
}

type CopyPod struct{}

var copyPod CopyPod

func (c *CopyPod) GetList(ctx context.Context, cl client.Client, namespace string) ([]client.Object, error) {
	pods := v1core.PodList{}
	err := cl.List(ctx, &pods, &client.ListOptions{Namespace: namespace})
	if err != nil {
		return nil, err
	}
	objects := []client.Object{}
	for _, pod := range pods.Items {
		if len(pod.GetOwnerReferences()) == 0 {
			objects = append(objects, &pod)
		}
	}
	return objects, nil
}

func (c *CopyPod) CompareSpec(src, dest client.Object) bool {
	return reflect.DeepEqual(src.(*v1core.Pod).Spec, dest.(*v1core.Pod))
}

func (c *CopyPod) CopySpec(src, dest client.Object) {
	dest.(*v1core.Pod).Spec = src.(*v1core.Pod).Spec
}
