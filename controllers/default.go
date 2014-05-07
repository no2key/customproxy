package controllers

import (
	"encoding/json"
	"fmt"
	"github.com/astaxie/beego"
	"github.com/fzzy/radix/redis"
	"os"
	"stack/proxy"
	"strings"
	"time"
)

func errHndlr(err error) {
	if err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
}

type MainController struct {
	beego.Controller
}

func (this *MainController) Get() {
	this.Data["Website"] = "beego.me"
	this.Data["Email"] = "astaxie@gmail.com"
	this.TplNames = "index.tpl"
}

type UrlController struct {
	beego.Controller
}

func (this *UrlController) View() {
	mp := make(map[string]interface{})
	id := this.Ctx.Input.Param("0")
	rds, err := redis.DialTimeout("tcp", "127.0.0.1:6379", time.Duration(10)*time.Second)
	errHndlr(err)
	defer rds.Close()
	sn := id
	//将当前请求的URL记录在redis里 key就等于 ulr-{id}
	url := rds.Cmd("GET", fmt.Sprintf("url-%s", sn)).String()
	content := rds.Cmd("GET", fmt.Sprintf("content-%s", sn)).String()
	headersStr := rds.Cmd("GET", fmt.Sprintf("headers-%s", sn)).String()
	mp["url"] = url
	mp["id"] = id
	mp["body"] = content
	mp["domain"] = "这是请求内容"
	var headers map[string]interface{}
	err = json.Unmarshal([]byte(headersStr), &headers)
	mp["headers"] = headers
	this.Data["json"] = mp
	this.ServeJson()
}

func (this *UrlController) ListTpl() {
	//this.ServeJson()
	this.TplNames = "list.tpl"
}

func (this *UrlController) DetailTpl() {
	this.TplNames = "detail.tpl"
}
func (this *UrlController) IPS() {
	proxy.PushActiveClient("127.0.0.1:6379", this.Ctx)
	ips := proxy.GetActiveClients("127.0.0.1:6379")
	this.Data["json"] = ips
	this.ServeJson()
}

func (this *UrlController) Myip() {
	this.Data["json"] = this.Ctx.Request.RemoteAddr
	this.ServeJson()
}

func (this *UrlController) CleanIp() {
	proxy.CleanIP("127.0.0.1:6379", strings.Split(this.Ctx.Request.RemoteAddr, ":")[0])
	this.Data["json"] = "OK"
	this.ServeJson()
}

/**
客户端连接的时候,将IP记录到系统库中(记录到redis中去)
*/
func (this *UrlController) List() {
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
	urls := proxy.FetchUrlList4Ip("127.0.0.1:6379", "127.0.0.1", 100)
	// l := len(urls)
	// mp := make([]map[string]interface{}, l)
	// j := 0
	// for _, v := range urls {
	// 	tmp := make(map[string]interface{})
	// 	tmp["url"] = v
	// 	tmp["id"] = "nothingid"
	// 	mp[j] = tmp
	// 	j++
	// }
	//mp := []interface{}
	//mp["key"] = proxy.GetActiveClients("127.0.0.1:6379")
	//mp["urls"] = proxy.FetchUrlList4Ip("127.0.0.1:6379", "127.0.0.1")

	this.Data["json"] = urls

	this.ServeJson()
}
