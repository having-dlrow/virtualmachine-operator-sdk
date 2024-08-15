package api

import (
	"fmt"
	"github.com/example/virtualmachine/api/v1"
	"github.com/example/virtualmachine/internal/controller"
	"github.com/example/virtualmachine/internal/model"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/log"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"net/http"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"time"
)

var (
	apiLog = ctrl.Log.WithName("api")
)

type VmController struct {
	k8sClient client.Client
}

func NewVmController(k8sClient client.Client) *VmController {
	return &VmController{k8sClient: k8sClient}
}

// @Summary		VirtualMachine 조회
// @Description	VirtualMachine 정보를 조회한다.
// @Accept			json
// @Produce		json
// @Param			id	path	string	true	"Virtual Machine ID"
// @Success		200
// @Failure		404
// @Router			/virtualmachines/{id} [get]
func (c *VmController) GetVM(ctx echo.Context) error {
	// 요청 데이터
	id := ctx.Param("id")
	if id == "" {
		return ctx.JSON(http.StatusBadRequest, map[string]string{"error": "ID parameter is required"})
	}

	vm, err, done := c.getVM(ctx, id)
	if done {
		return err
	}

	// Iterate over the list and print the names of the VirtualMachines
	apiLog.Info("VirtualMachine", "name", vm.Name, "namespace", vm.Namespace, "status", vm.Status)

	return ctx.JSON(http.StatusOK, vm)
}

func (c *VmController) getVM(ctx echo.Context, id string) (*v1.VirtualMachine, error, bool) {
	apiLog.Info("Fetching VirtualMachine", "id", id)

	// Define the NamespacedName for the VirtualMachine
	namespacedName := types.NamespacedName{
		Namespace: "default", // Use the appropriate namespace or extract from request if needed
		Name:      id,
	}

	// Fetch the list of VirtualMachines
	vm := &v1.VirtualMachine{}
	if err := c.k8sClient.Get(ctx.Request().Context(), namespacedName, vm); err != nil {
		if errors.IsNotFound(err) {
			apiLog.Info("VirtualMachine not found", "vm", vm.Finalizers)
			return nil, ctx.JSON(http.StatusNotFound, map[string]string{"error": "VirtualMachine not found"}), false
		}
		return nil, ctx.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to get VirtualMachine"}), true
	}
	apiLog.Info("was ok", "vm", vm.Finalizers)
	return vm, nil, false
}

// @Summary		VirtualMachine 목록
// @Description	VirtualMachine 목록를 조회한다.
// @Accept			json
// @Produce		json
// @Success		200
// @Failure		404
// @Router			/virtualmachines [get]
func (c *VmController) ListVM(ctx echo.Context) error {

	vmList := &v1.VirtualMachineList{}
	err := c.k8sClient.List(ctx.Request().Context(), vmList, &client.ListOptions{
		Namespace: "default", // Replace with the desired namespace or leave empty to list all namespaces
	})

	if err != nil {
		apiLog.Error(err, "Failed to list VirtualMachines")
	} else {
		// Iterate over the list and print the names of the VirtualMachines
		for _, vm := range vmList.Items {
			apiLog.Info("VirtualMachine", "name", vm.Name, "namespace", vm.Namespace, "status", vm.Status)
		}
	}

	return ctx.JSON(http.StatusOK, vmList)
}

// @Summary		VirtualMachine 생성
// @Description	VirtualMachine 정보를 생성한다.
// @Accept			json
// @Produce		json
// @Param			vm	body	model.VMRequest	true	"Request of VirtualMachine"
// @Success		202
// @Failure		404
// @Router			/virtualmachines [post]
func (c *VmController) CreateVM(ctx echo.Context) error {
	// Bind the request body to a VirtualMachineSpec
	req := new(model.VMRequest)
	if err := ctx.Bind(req); err != nil {
		apiLog.Error(err, "Failed to bind request body to VirtualMachineSpec")
		return ctx.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request payload"})
	}

	// Create a new VirtualMachine object
	vm := &v1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: "default", // Set the default namespace or use vm.Namespace if available
		},
		Spec: req.Spec,
		Status: v1.VirtualMachineStatus{
			UUID:      generateUUID(),                  // Assuming generateUUID is a function that generates a unique identifier
			DevAdmin:  req.Spec.DevAdmin,               // Copy DevAdmin from Spec to Status
			Status:    "Creating",                      // Set initial status
			Hostname:  req.Spec.Image,                  // Example: Generating hostname based on the image name
			CreatedAt: time.Now().Format(time.RFC3339), // Set the creation timestamp
		},
	}

	// Set the namespace if not set (assuming a default namespace for this example)
	if vm.Namespace == "" {
		vm.Namespace = "default"
	}

	// Log the creation request
	apiLog.Info("Creating new VirtualMachine", "name", vm.Name, "namespace", vm.Namespace)

	// Attempt to create the VirtualMachine resource
	if err := c.k8sClient.Create(ctx.Request().Context(), vm); err != nil {
		apiLog.Error(err, "Failed to create VirtualMachine", "name", vm.Name)
		return ctx.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create VirtualMachine"})
	}

	// Log the successful creation
	apiLog.Info("Successfully created VirtualMachine", "name", vm.Name, "namespace", vm.Namespace)

	// Return the created VirtualMachine as a JSON response
	return ctx.JSON(http.StatusCreated, vm)
}

// @Summary		VirtualMachine 삭제
// @Description	VirtualMachine 삭제합니다.
// @Accept			json
// @Produce		json
// @Param			id	path	string	true	"Virtual Machine ID"
// @Success		204
// @Failure		404
// @Router			/virtualmachines/{id} [delete]
func (c *VmController) DeleteVM(ctx echo.Context) error {

	// 요청 데이터
	id := ctx.Param("id")

	// fetch the VirtualMachine Custom Resource before updating the status
	vm, err, done := c.getVM(ctx, id)
	if vm == nil {
		log.Error(err, "Failed to re-fetch VirtualMachine")
		return err
	}

	meta.SetStatusCondition(&vm.Status.Conditions, metav1.Condition{Type: controller.TypeDeletingVirtualMachine,
		Status: metav1.ConditionTrue, Reason: "Finalizing",
		Message: fmt.Sprintf("Finalizer operations for custom resource %s name were successfully accomplished", vm.Name)})

	apiLog.Info("update", "vm", vm.Status.Conditions)
	if err := c.k8sClient.Update(ctx.Request().Context(), vm); err != nil {
		log.Error(err, "Failed to update VirtualMachine status")
		return err
	}

	apiLog.Info("remove finalizer", "vm", vm.Finalizers)
	if ok := controllerutil.RemoveFinalizer(vm, controller.FinalizerNameVirtualMachine); !ok {
		log.Info("Failed to remove finalizer for VirtualMachine")
		return nil
	}

	apiLog.Info("remove finalizer & update", "vm", vm.Finalizers)
	if err := c.k8sClient.Update(ctx.Request().Context(), vm); err != nil {
		apiLog.Error(err, "Failed to remove finalizer for VirtualMachine")
		return err
	}

	// Re-fetch the VirtualMachine Custom Resource before Deleting
	vm, err, done = c.getVM(ctx, id)
	if !done {
		return ctx.NoContent(http.StatusNoContent)
	}

	if vm != nil {
		apiLog.Info("delete", "vm", vm.Name)
		if err := c.k8sClient.Delete(ctx.Request().Context(), vm); err != nil {
			log.Error(err, "Failed to remove VirtualMachine")
			return err
		}
	}

	return ctx.NoContent(http.StatusNoContent)
}

func generateUUID() string {
	return uuid.New().String()
}
