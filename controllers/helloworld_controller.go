/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	"time"

	"github.com/example/memcached-operator/api/v1alpha1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	svcImage      = "gcr.io/run-ai-lab/dans-service"
	annotationKey = "hw_name"
	svcPostfix    = "-service"
)

// HelloworldReconciler reconciles a Helloworld object
type HelloworldReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=cache.my.domain,resources=helloworlds,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cache.my.domain,resources=helloworlds/status,verbs=get;update;patch;create
//+kubebuilder:rbac:groups=cache.my.domain,resources=helloworlds/finalizers,verbs=update

//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete

func (r *HelloworldReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	Logging(req, "Starting reconcile")

	helloworld := &v1alpha1.Helloworld{}

	if err := r.Get(ctx, req.NamespacedName, helloworld); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		log.Log.Error(err, "unable to fetch Helloworld")
		return ctrl.Result{}, err
	}

	svcCreated, err := r.validateOrCreateSvcFor(helloworld, ctx, req)
	if err != nil {
		return ctrl.Result{}, err
	}
	if svcCreated {
		return ctrl.Result{Requeue: true}, nil
	}

	podsCreated, err := r.validateOrCreatePodsFor(helloworld, ctx, req)
	if err != nil {
		return ctrl.Result{}, err
	}
	if podsCreated {
		return ctrl.Result{Requeue: true}, nil
	}

	Logging(req, "Finished succesfully")

	return ctrl.Result{Requeue: false}, nil
}

func (r *HelloworldReconciler) validateOrCreateSvcFor(helloworld *v1alpha1.Helloworld, ctx context.Context, req ctrl.Request) (bool, error) {
	svc := &v1.Service{}
	svcName := getSvcName(helloworld.Name)

	svcNamespacedName := types.NamespacedName{Namespace: helloworld.Namespace, Name: svcName}
	svcCreated := true

	if err := r.Get(ctx, svcNamespacedName, svc); err != nil {
		if errors.IsNotFound(err) {
			Logging(req, "Creating a service")
			svc, err := r.defineSvcFor(helloworld)
			if err != nil {
				return !svcCreated, err
			}

			if err = r.Create(ctx, svc); err != nil {
				return !svcCreated, err
			}
			return svcCreated, nil
		}
		log.Log.Error(err, fmt.Sprintf("unable to fetch %s's service", helloworld.Name))
		return !svcCreated, err
	}

	Logging(req, fmt.Sprintf("Service exist - %s", svc.Name))
	return !svcCreated, nil
}

func getSvcName(helloworldName string) string {
	return helloworldName + svcPostfix
}

func (r *HelloworldReconciler) defineSvcFor(helloworld *v1alpha1.Helloworld) (*v1.Service, error) {
	labels := labelsForApp(helloworld.Name)
	svcName := getSvcName(helloworld.Name)

	svc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      svcName,
			Namespace: helloworld.Namespace,
			Labels:    labels,
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{
					Protocol:   v1.ProtocolTCP,
					Port:       8080,
					TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: 8080},
				}},
			Selector: labels,
			Type:     v1.ServiceTypeClusterIP,
		},
	}

	if err := controllerutil.SetControllerReference(helloworld, svc, r.Scheme); err != nil {
		return svc, err
	}

	return svc, nil
}

func (r *HelloworldReconciler) validateOrCreatePodsFor(helloworld *v1alpha1.Helloworld, ctx context.Context, req ctrl.Request) (bool, error) {
	requiredReplicas := helloworld.Spec.ReplicaCount
	podsCreated := true

	Logging(req, fmt.Sprintf("Required count of pods is %d", requiredReplicas))

	podList := &v1.PodList{}
	opts := []client.ListOption{
		client.InNamespace(helloworld.Namespace),
		client.MatchingLabels{annotationKey: helloworld.Name},
	}

	if err := r.List(ctx, podList, opts...); err != nil {
		log.Log.Error(err, fmt.Sprintf("unable to list %s's pods", helloworld.Name))
		return !podsCreated, err
	}

	currentReplicas := len(podList.Items)
	Logging(req, fmt.Sprintf("Current count of pods is %d, desired is %d", currentReplicas, requiredReplicas))

	if requiredReplicas > int32(currentReplicas) {
		for i := int32(currentReplicas); i < requiredReplicas; i++ {
			Logging(req, fmt.Sprintf("Creating a pod number %d", i))
			pod, err := r.definePodFor(helloworld)
			if err != nil {
				return !podsCreated, err
			}

			if err := r.Create(ctx, pod); err != nil {
				return !podsCreated, err
			}
		}

		return podsCreated, nil
	}

	return !podsCreated, r.updateStatusFor(helloworld, podList, ctx, req)
}

func (r *HelloworldReconciler) updateStatusFor(helloworld *v1alpha1.Helloworld, podList *v1.PodList, ctx context.Context, req ctrl.Request) error {
	countReady := 0
	for _, pod := range podList.Items {
		conditions := pod.Status.Conditions
		for _, condition := range conditions {
			if condition.Type == "Ready" && condition.Status == "True" {
				countReady++
			}
		}
	}

	helloworld.Status.AvailableReplicas = int32(len(podList.Items))
	helloworld.Status.ReadyReplicas = int32(countReady)

	Logging(req, "Updating status")

	if err := r.Status().Update(ctx, helloworld); err != nil {
		log.Log.Error(err, "unable to update Helloworld")
		return err
	}

	return nil
}

func (r *HelloworldReconciler) definePodFor(helloworld *v1alpha1.Helloworld) (*v1.Pod, error) {
	labels := labelsForApp(helloworld.Name)
	podName := fmt.Sprintf("%s-pod-%d", helloworld.Name, time.Now().UnixNano())

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: helloworld.Namespace,
			Labels:    labels,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{{
				Name:  fmt.Sprintf("%s-contatiner", podName),
				Image: svcImage,
				Env: []v1.EnvVar{{
					Name:  "content",
					Value: helloworld.Spec.DefaultContent,
				}},
			}},
		},
	}

	if err := controllerutil.SetControllerReference(helloworld, pod, r.Scheme); err != nil {
		return pod, err
	}

	return pod, nil
}

func labelsForApp(name string) map[string]string {
	return map[string]string{annotationKey: name}
}

// SetupWithManager sets up the controller with the Manager.
func (r *HelloworldReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Helloworld{}).
		Owns(&v1.Service{}).
		Owns(&v1.Pod{}).
		Complete(r)
}

func Logging(req ctrl.Request, msg string) {
	log.Log.Info(fmt.Sprintf("%s - %s", req.Name, msg))
}
