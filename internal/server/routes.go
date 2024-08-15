package server

import (
	"github.com/example/virtualmachine/internal/api"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func (s Server) InitRoutes(e *echo.Echo) {

	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept},
	}))

	controller := s.InitVmController()
	GroupController := s.InitVmGroupController()

	// api
	// virtual machine
	e.POST("/virtualmachines", controller.CreateVM)
	e.GET("/virtualmachines/:id", controller.GetVM)
	e.GET("/virtualmachines", controller.ListVM)
	e.DELETE("/virtualmachines/:id", controller.DeleteVM)

	// virtual machine group
	e.GET("/virtualmachinegroups/:id", GroupController.GetVMGroup)
	e.GET("/virtualmachinegroups", GroupController.ListVMGroup)

}

func (s Server) InitVmController() *api.VmController {
	return api.NewVmController(s.k8sClient)
}

func (s Server) InitVmGroupController() *api.VmGroupController {
	return api.NewVmGroupController(s.k8sClient)
}
