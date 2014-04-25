package main

import (
	//	"fmt"
	"github.com/astaxie/beego"
	"stack/controllers"
	"stack/proxy"
)

func main() {
	go proxy.RunProxy()
	beego.Router("/", &controllers.MainController{})
	beego.AutoRouter(&controllers.UrlController{})
	beego.SetStaticPath("/static", "static")
	beego.TemplateLeft = "<<<"
	beego.TemplateRight = ">>>"

	beego.Run()
}
