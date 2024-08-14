package api

import (
	"github.com/labstack/echo/v4"
	"net/http"
)

type VmController struct {
}

func NewVmController() *VmController {
	return &VmController{}
}

func (c *VmController) GetVM(ctx echo.Context) error {
	// 요청 데이터
	_ = ctx.Param("id")
	//namespacedName := types.NamespacedName{
	//	Namespace: "default",
	//	Name:      id,
	//}
	//
	//vm := &computev1.VirtualMachine{}
	//if err := c.Get(ctx.Request().Context(), namespacedName, vm); err != nil {
	//	return client.IgnoreNotFound(err)
	//}

	return ctx.JSON(http.StatusOK, "ok")
}
