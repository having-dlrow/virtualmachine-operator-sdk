package api

import (
	"github.com/example/virtualmachine/api/v1"
	"github.com/labstack/echo/v4"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type VmGroupController struct {
	k8sClient client.Client
}

func NewVmGroupController(k8sClient client.Client) *VmGroupController {
	return &VmGroupController{k8sClient: k8sClient}
}

// @Summary		VirtualMachineGroup 조회
// @Description	VirtualMachineGroup 정보를 조회한다.
// @Accept			json
// @Produce		json
// @Param			id	path	string	true	"Virtual Machine ID"
// @Success		200
// @Failure		404
// @Router			/virtualmachinegroups/{id} [get]
func (vc *VmGroupController) GetVMGroup(ctx echo.Context) error {
	// 요청 데이터
	id := ctx.Param("id")
	if id == "" {
		return ctx.JSON(http.StatusBadRequest, map[string]string{"error": "ID parameter is required"})
	}

	vm, err2, done := vc.getVMGroup(ctx, id)
	if done {
		return err2
	}

	// Iterate over the list and print the names of the VirtualMachines
	apiLog.Info("VirtualMachine", "name", vm.Name, "namespace", vm.Namespace, "status", vm.Status)

	return ctx.JSON(http.StatusOK, vm)
}

func (c *VmGroupController) getVMGroup(ctx echo.Context, id string) (*v1.VirtualMachineGroup, error, bool) {
	apiLog.Info("Fetching VirtualMachineGroup", "id", id)

	// Define the NamespacedName for the VirtualMachine
	namespacedName := types.NamespacedName{
		Namespace: "default", // Use the appropriate namespace or extract from request if needed
		Name:      id,
	}

	vm := &v1.VirtualMachineGroup{}
	if err := c.k8sClient.Get(ctx.Request().Context(), namespacedName, vm); err != nil {

		if errors.IsNotFound(err) {
			return nil, ctx.JSON(http.StatusNotFound, map[string]string{"error": "VirtualMachineGroup not found"}), true
		}
		return nil, ctx.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to get VirtualMachineGroup"}), true
	}
	return vm, nil, false
}

// @Summary		VirtualGroupMachine 목록
// @Description	VirtualGroupMachine 목록를 조회한다.
// @Accept			json
// @Produce		json
// @Success		200
// @Failure		404
// @Router			/virtualmachinegroups [get]
func (c *VmGroupController) ListVMGroup(ctx echo.Context) error {

	vmList := &v1.VirtualMachineGroupList{}
	err := c.k8sClient.List(ctx.Request().Context(), vmList, &client.ListOptions{
		Namespace: "default", // Replace with the desired namespace or leave empty to list all namespaces
	})

	if err != nil {
		apiLog.Error(err, "Failed to list VirtualMachineGroup")
	} else {
		// Iterate over the list and print the names of the VirtualMachines
		for _, vm := range vmList.Items {
			apiLog.Info("VirtualMachineGroup", "name", vm.Name, "namespace", vm.Namespace, "status", vm.Status)
		}
	}

	return ctx.JSON(http.StatusOK, vmList)
}
