package server

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func (s Server) InitRoutes(e *echo.Echo) {

	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept},
	}))

	//controller := s.InitVmController()

	// api controller
	//e.POST("/virtualmachines", vmController.Create)
	//e.GET("/virtualmachines/:id", controller.GetVM)
	//e.GET("/virtualmachines", vmController.List)
	//e.DELETE("/virtualmachines/:id", vmController.Delete)
}

//func (s Server) InitVmController() *api.VmController {
//	return api.NewVmController(s.k8sClient)
//}
