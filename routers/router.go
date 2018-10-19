// @APIVersion 1.0.0
// @Title beego Test API
// @Description beego has a very cool tools to autogenerate documents for your API
// @Contact astaxie@gmail.com
// @TermsOfServiceUrl http://beego.me/
// @License Apache 2.0
// @LicenseUrl http://www.apache.org/licenses/LICENSE-2.0.html
package routers

import (
	"github.com/astaxie/beego"
	"github.com/cdvr1993/deployment-manager/controllers"
	"github.com/cdvr1993/deployment-manager/services"
)

type ServiceManager struct {
	GroupService services.IGroupService
	UserService  services.IUserService
}

func InitRouter(m ServiceManager) {
	ns := beego.NewNamespace("/v1",
		beego.NSNamespace("/group",
			beego.NSInclude(
				&controllers.GroupController{
					GroupService: m.GroupService,
				},
			),
		),
		beego.NSNamespace("/user",
			beego.NSInclude(
				&controllers.UserController{
					UserService: m.UserService,
				},
			),
		),
	)
	beego.AddNamespace(ns)
}

func InjectServices() {
	// Used as a dependency injection
	InitRouter(ServiceManager{
		GroupService: services.NewGroupService(),
		UserService:  services.NewUserService(),
	})
}
