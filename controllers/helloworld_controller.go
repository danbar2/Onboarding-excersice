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

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Helloworld object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
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

	svc := &v1.Service{}
	svcNamespacedName := types.NamespacedName{Namespace: helloworld.Namespace, Name: helloworld.Name + "-service"}

	if err := r.Get(ctx, svcNamespacedName, svc); err != nil {
		if errors.IsNotFound(err) {
			Logging(req, "Creating a service")
			r.Create(ctx, r.svcForHelloworld(helloworld))
			return ctrl.Result{Requeue: true}, nil
		}
		log.Log.Error(err, "unable to fetch "+helloworld.Name+"'s service")
		return ctrl.Result{}, err
	}

	Logging(req, "Service exist - "+svc.Name)

	requiredReplicas := helloworld.Spec.ReplicaCount

	Logging(req, "Required count of pods is "+fmt.Sprint(requiredReplicas))

	podList := &v1.PodList{}
	opts := []client.ListOption{
		client.InNamespace(helloworld.Namespace),
		client.MatchingLabels{"hw_name": helloworld.Name},
	}

	if err := r.List(ctx, podList, opts...); err != nil {
		log.Log.Error(err, "unable to list "+helloworld.Name+"'s pods")
		return ctrl.Result{}, err
	}

	currentReplicas := len(podList.Items)
	Logging(req, "Current count of pods is "+fmt.Sprint(currentReplicas))

	if requiredReplicas > int32(currentReplicas) {
		for i := int32(0); i < requiredReplicas-int32(currentReplicas); i++ {
			Logging(req, "Creating a pod number "+fmt.Sprint(i))
			r.Create(ctx, r.podForHelloworld(helloworld))
		}

		return ctrl.Result{Requeue: true}, nil
	}

	countReady := 0
	for _, pod := range podList.Items {
		conditions := pod.Status.Conditions
		for _, condition := range conditions {
			if condition.Type == "Ready" && condition.Status == "True" {
				countReady++
			}
		}
	}

	helloworld.Status.AvailableReplicas = int32(currentReplicas)
	helloworld.Status.ReadyReplicas = int32(countReady)

	Logging(req, "Updating status")

	if err := r.Status().Update(ctx, helloworld); err != nil {
		log.Log.Error(err, "unable to update Helloworld")
		return ctrl.Result{}, err
	}

	Logging(req, "Finished succesfully")

	return ctrl.Result{Requeue: false}, nil
}

func (r *HelloworldReconciler) svcForHelloworld(h *v1alpha1.Helloworld) *v1.Service {
	lbls := labelsForApp(h.Name)

	svc := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      h.Name + "-service",
			Namespace: h.Namespace,
			Labels:    lbls,
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{
					Protocol:   v1.ProtocolTCP,
					Port:       8080,
					TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: 8080},
				}},
			Selector: lbls,
			Type:     v1.ServiceTypeClusterIP,
		},
	}

	controllerutil.SetControllerReference(h, svc, r.Scheme)
	return svc
}

func (r *HelloworldReconciler) podForHelloworld(h *v1alpha1.Helloworld) *v1.Pod {
	lbls := labelsForApp(h.Name)
	podName := h.Name + "-pod-" + fmt.Sprint(time.Now().UnixNano())

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: h.Namespace,
			Labels:    lbls,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{{Name: podName + "-contatiner", Image: "gcr.io/run-ai-lab/dans-service", Env: []v1.EnvVar{{Name: "content", Value: h.Spec.DefaultContent}}}},
		},
	}

	controllerutil.SetControllerReference(h, pod, r.Scheme)
	return pod
}

func labelsForApp(name string) map[string]string {
	return map[string]string{"hw_name": name}
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
	log.Log.Info(req.Name + " - " + msg)
}
