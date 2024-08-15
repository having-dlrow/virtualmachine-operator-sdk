/*
Copyright 2024.

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

package controller

import (
	"context"
	"fmt"
	vmv1 "github.com/example/virtualmachine/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"net/http"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"strings"
	"time"
)

const (
	FinalizerNameVirtualMachine = "vm.example.com/VirtualMachine"
	// typeAvailableMemcached represents the status of the Deployment reconciliation
	TypeAvailableVirtualMachine = "Available"
	// typeDegradedVirtualMachine represents the status used when the custom resource is deleted and the finalizer operations are yet to occur.
	TypeDeletingVirtualMachine = "Deleting"
)

// VirtualMachineReconciler reconciles a VirtualMachine object
type VirtualMachineReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=vm.example.com,resources=virtualmachines,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=vm.example.com,resources=virtualmachines/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=vm.example.com,resources=virtualmachines/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch

func (r *VirtualMachineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the VirtualMachine instance
	vm := &vmv1.VirtualMachine{}
	err := r.Get(ctx, req.NamespacedName, vm)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("VirtualMachine resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get VirtualMachine")
		return ctrl.Result{}, err
	}

	// Let's just set the status as Unknown when no status is available
	if vm.Status.Conditions == nil || len(vm.Status.Conditions) == 0 {
		meta.SetStatusCondition(&vm.Status.Conditions, metav1.Condition{Type: TypeAvailableVirtualMachine, Status: metav1.ConditionUnknown, Reason: "Reconciling", Message: "Starting reconciliation"})
		if err := r.Status().Update(ctx, vm); err != nil {
			log.Error(err, "Failed to update VirtualMachine status")
			return ctrl.Result{}, err
		}
		// Let's re-fetch the VirtualMachine Custom Resource after updating the status
		if err := r.Get(ctx, req.NamespacedName, vm); err != nil {
			log.Error(err, "Failed to re-fetch VirtualMachine")
			return ctrl.Result{}, err
		}
	}

	// Let's add a finalizer.
	if !controllerutil.ContainsFinalizer(vm, FinalizerNameVirtualMachine) {
		log.Info("Adding Finalizer for VirtualMachine")
		if ok := controllerutil.AddFinalizer(vm, FinalizerNameVirtualMachine); !ok {
			log.Error(err, "Failed to add finalizer into the custom resource")
			return ctrl.Result{Requeue: true}, nil
		}

		if err := r.Update(ctx, vm); err != nil {
			log.Error(err, "Failed to update custom resource to add finalizer")
			return ctrl.Result{}, err
		}
	}

	// Check if the VirtualMachine instance is marked to be deleted
	isVirtualMachineMarkedToBeDeleted := vm.GetDeletionTimestamp() != nil
	if isVirtualMachineMarkedToBeDeleted {
		if controllerutil.ContainsFinalizer(vm, FinalizerNameVirtualMachine) {
			log.Info("Performing Finalizer Operations for VirtualMachine before delete CR")

			// Let's add here a status "Deleting" to reflect that this resource began its process to be terminated.
			meta.SetStatusCondition(&vm.Status.Conditions, metav1.Condition{Type: TypeDeletingVirtualMachine,
				Status: metav1.ConditionUnknown, Reason: "Finalizing",
				Message: fmt.Sprintf("Performing finalizer operations for the custom resource: %s ", vm.Name)})

			if err := r.Status().Update(ctx, vm); err != nil {
				log.Error(err, "Failed to update VirtualMachine status")
				return ctrl.Result{}, err
			}

			// Delete API Call
			deleteVirtualMachineInAPI(vm)
		}
	}

	// Check if the deployment already exists, if not create a new one
	found := &appsv1.Deployment{}
	err = r.Get(ctx, types.NamespacedName{Name: vm.Name, Namespace: vm.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		// Define a new deployment
		dep, err := r.deploymentForVirtualMachine(vm)
		if err != nil {
			log.Error(err, "Failed to define new Deployment resource for VirtualMachine")

			// The following implementation will update the status
			meta.SetStatusCondition(&vm.Status.Conditions, metav1.Condition{Type: TypeAvailableVirtualMachine,
				Status: metav1.ConditionFalse, Reason: "Reconciling",
				Message: fmt.Sprintf("Failed to create Deployment for the custom resource (%s): (%s)", vm.Name, err)})

			if err := r.Status().Update(ctx, vm); err != nil {
				log.Error(err, "Failed to update VirtualMachine status")
				return ctrl.Result{}, err
			}

			return ctrl.Result{}, err
		}

		log.Info("Creating a new Deployment",
			"Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
		if err = r.Create(ctx, dep); err != nil {
			log.Error(err, "Failed to create new Deployment",
				"Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			return ctrl.Result{}, err
		}
	} else if err != nil {
		log.Error(err, "Failed to get Deployment")
		// Let's return the error for the reconciliation be re-trigged again
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// deploymentForVirtualMachine returns a VirtualMachine Deployment object
func (r *VirtualMachineReconciler) deploymentForVirtualMachine(vm *vmv1.VirtualMachine) (*appsv1.Deployment, error) {
	ls := labelsForVirtualMachine(vm)

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      vm.Name,
			Namespace: vm.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: ls,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: ls,
				},
				Spec: corev1.PodSpec{
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: &[]bool{true}[0],
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
					Containers: []corev1.Container{{
						Image:           vm.Spec.Image,
						Name:            vm.Name,
						ImagePullPolicy: corev1.PullIfNotPresent,
						Command:         []string{"/bin/bash", "-c", "--"}, // bash로 실행하도록 설정
						Args:            []string{"while true; do sleep 30; done;"},
						SecurityContext: &corev1.SecurityContext{
							RunAsNonRoot:             &[]bool{true}[0],
							RunAsUser:                &[]int64{1001}[0],
							AllowPrivilegeEscalation: &[]bool{false}[0],
							Capabilities: &corev1.Capabilities{
								Drop: []corev1.Capability{
									"ALL",
								},
							},
						},
					}},
				},
			},
		},
	}

	// Set the ownerRef for the Deployment
	if err := ctrl.SetControllerReference(vm, dep, r.Scheme); err != nil {
		return nil, err
	}
	return dep, nil
}

func labelsForVirtualMachine(vm *vmv1.VirtualMachine) map[string]string {
	imageTag := strings.Split(vm.Spec.Image, ":")[1]
	return map[string]string{"app.kubernetes.io/name": "v1.vm",
		"app.kubernetes.io/version":    imageTag,
		"app.kubernetes.io/app.name":   vm.Name,
		"app.kubernetes.io/managed-by": "vm.Controller",
	}
}

func deleteVirtualMachineInAPI(vm *vmv1.VirtualMachine) error {
	url := fmt.Sprintf("http://localhost:8888/virtualmachines/%s", vm.Name)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("failed to delete virtual machine, status code: %d", resp.StatusCode)
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *VirtualMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&vmv1.VirtualMachine{}).
		Complete(r)
}
