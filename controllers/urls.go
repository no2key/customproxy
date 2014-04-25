package controllers

import (
	"github.com/astaxie/beego"
	"stack/proxy"
	"strings"
)

type UrlsController struct {
	beego.Controller
}

/**
客户端连接的时候,将IP记录到系统库中(记录到redis中去)
*/
func (this *UrlsController) Get() {
	proxy.PushActiveClient("127.0.0.1:6379", this.Ctx)
	this.Data["Website"] = "beego.com"
	//this.Data["Ip"] =
	this.Data["Email"] = "astaxie@gmail.com"
	this.TplNames = "index.tpl"
	if proxy.IsActiveClients("127.0.0.1:6379", strings.Split(this.Ctx.Request.RemoteAddr, ":")[0]) {
		this.Data["IsClient"] = "yes is active client"
	} else {
		this.Data["IsClient"] = "no,it's not active client"
	}
	urls := proxy.FetchUrlList4Ip("127.0.0.1:6379", "127.0.0.1")
	l := len(urls)
	mp := make([]map[string]interface{}, l)
	j := 0
	for _, v := range urls {
		tmp := make(map[string]interface{})
		tmp["url"] = v
		tmp["id"] = "nothingid"
		mp[j] = tmp
		j++
	}
	//mp := []interface{}
	//mp["key"] = proxy.GetActiveClients("127.0.0.1:6379")
	//mp["urls"] = proxy.FetchUrlList4Ip("127.0.0.1:6379", "127.0.0.1")

	this.Data["json"] = mp

	this.ServeJson()
}
