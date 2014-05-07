package proxy

import (
	"fmt"
	//"github.com/astaxie/beego"
	"github.com/astaxie/beego/context"
	"github.com/elazarl/goproxy"
	//"github.com/elazarl/goproxy/ext/html"
	"encoding/json"
	"github.com/fzzy/radix/redis"
	"io"
	"log"
	. "net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type Identifier struct {
	Url string
	Sn  uint64
	Ip  string
}
type CountReadCloser struct {
	Id          string
	R           io.ReadCloser
	nr          uint64
	content     []byte //a copy of content
	Sn          uint64 //request number
	redisConfig string //redis config
}

func (c *CountReadCloser) Read(b []byte) (n int, err error) {
	n, err = c.R.Read(b)
	c.content = append(c.content[0:c.nr], b[0:n]...)
	c.nr += uint64(n)
	return
}
func (c CountReadCloser) Close() error {
	SaveUrlContentInRedis(c, c.redisConfig)
	return c.R.Close()
}

func SaveUrlContentInRedis(cr CountReadCloser, redisConfig string) {
	rds, err := redis.DialTimeout("tcp", redisConfig, time.Duration(10)*time.Second)
	errHndlr(err)
	defer rds.Close()
	sn := strconv.FormatUint(cr.Sn, 36)
	//将当前请求的内容记录在redis里 key就等于 content-{id}
	rds.Cmd("SET", fmt.Sprintf("content-%s", sn), string(cr.content))
}

func errHndlr(err error) {
	if err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
}
func holdRedis(req_chan chan *Identifier, redisConfig string) {
	rds, err := redis.DialTimeout("tcp", redisConfig, time.Duration(10)*time.Second)
	errHndlr(err)
	defer rds.Close()
	for {
		select {
		case cnt := <-req_chan:
			s := rds.Cmd("lpush", cnt.Ip, strconv.FormatUint(cnt.Sn, 36))
			if s == nil {
			} else {
				fmt.Println("push into list", cnt.Ip, " 失败,", s)
			}
		}
	}
}

/**
* 从channel里接收IP,并加入到活跃IP里去
 */
func holdActiveClients(ip_chan chan string) {
	rds, err := redis.DialTimeout("tcp", "127.0.0.1:6379", time.Duration(10)*time.Second)
	errHndlr(err)
	defer rds.Close()
	for {
		select {
		case ip := <-ip_chan:
			s := rds.Cmd("SADD", "activeIPs", ip)
			if s == nil {
			}
		}
	}
}

/**
* 从channel里接收IP,并加入到活跃IP里去
 */
func GetActiveClients(redisConfig string) []string {
	rds, err := redis.DialTimeout("tcp", redisConfig, time.Duration(10)*time.Second)
	errHndlr(err)
	defer rds.Close()
	s, _ := rds.Cmd("SMEMBERS", "activeIPs").List()
	return s
}

/**
 *  清理不再有持续连接的IP,避免记录太多不必要的数据,将redis撑爆表
 * 目前设定的是10分钟,10分钟内客户端没有过来操作,则视为不再记录信息;
 * @todo 可以改进一下,10分钟如果没有动作,则不再录制其访问信息,但是已经录制的,半小时后再删除;
 */
func RemoveActiveClients(redisConfig string) {
	var expired_seconds int64
	expired_seconds = 600
	rds, err := redis.DialTimeout("tcp", redisConfig, time.Duration(10)*time.Second)
	errHndlr(err)
	defer rds.Close()
	for {
		time.Sleep(time.Second * 10)
		now := time.Now().Unix()
		s, _ := rds.Cmd("SMEMBERS", "activeIPs").List()
		for _, v := range s {
			expire, err := rds.Cmd("GET", fmt.Sprintf("expire-%s", v)).Str()
			if err == nil {
				unixTimeStamp, er := strconv.ParseInt(expire, 36, 64)
				if er == nil {
					fmt.Println("got new time stamp:", unixTimeStamp)
					if (now - unixTimeStamp) > expired_seconds {
						fmt.Println("expired ,", unixTimeStamp)
						CleanIP(redisConfig, v)
					}
				}
			}
		}
	}
}

/**
* 清理为某一个IP记录的数据,包括该IP访问过的地址,所有地址的内容等等;
 */
func CleanIP(redisConfig string, ip string) {
	rds, err := redis.DialTimeout("tcp", redisConfig, time.Duration(10)*time.Second)
	errHndlr(err)
	defer rds.Close()
	s, _ := rds.Cmd("LRANGE", ip, 0, 10000).List()
	if s != nil {
		//有脏数据没有清理;
		fmt.Println("该IP没有清理数据:", ip)
	}

	for _, v := range s {
		rds.Cmd("DEL", fmt.Sprintf("url-%s", v), fmt.Sprintf("content-%s", v),
			fmt.Sprintf("headers-%s", v))
	}
	rds.Cmd("DEL", fmt.Sprintf("expire-%s", ip))
	rds.Cmd("SREM", "activeIPs", ip)
	rds.Cmd("LTRIM", ip, 1, 0)
}

/**
* 从channel里接收IP,并加入到活跃IP里去
 */
func IsActiveClients(redisConfig string, ip string) bool {
	rds, err := redis.DialTimeout("tcp", redisConfig, time.Duration(10)*time.Second)
	errHndlr(err)
	defer rds.Close()
	s, _ := rds.Cmd("SISMEMBER", "activeIPs", ip).Bool()
	return s
}

/**
 * 将新的IP记录到一个列表;
 * @todo 支持ipv6
 */
func PushActiveClient(redisConfig string, ctx *context.Context) {
	fmt.Println("ip:", ctx.Request.RemoteAddr)
	ip := strings.Split(ctx.Request.RemoteAddr, ":")[0]
	if ip == "[" {
		ip = "127.0.0.1"
	}
	rds, err := redis.DialTimeout("tcp", redisConfig, time.Duration(10)*time.Second)
	errHndlr(err)
	defer rds.Close()
	fmt.Println("add ip:", ip)
	s := rds.Cmd("SADD", "activeIPs", ip)
	rds.Cmd("SET", fmt.Sprintf("expire-%s", ip), strconv.FormatInt(time.Now().Unix(), 36))
	fmt.Println("sadd members:", s)
}

/**
* 取回某个IP 通过代理请求的所有url信息;
 */
func FetchUrlList4Ip(redisConfig string, ip string, size int) []map[string]interface{} {
	rds, err := redis.DialTimeout("tcp", redisConfig, time.Duration(10)*time.Second)
	errHndlr(err)
	defer rds.Close()
	s, _ := rds.Cmd("LRANGE", ip, 0, size).List()
	if s == nil {
		return make([]map[string]interface{}, 0)
	}
	res := make([]map[string]interface{}, len(s))
	i := 0
	for _, v := range s {
		tmp := make(map[string]interface{})
		url, _ := rds.Cmd("GET", fmt.Sprintf("url-%s", v)).Str()
		tmp["url"] = url
		tmp["id"] = v
		res[i] = tmp
		i++

	}
	return res
}

/**
某个IP是否以某个地址开头;
*/
func SrcIpBeginWith(ip string) goproxy.ReqCondition {
	return goproxy.ReqConditionFunc(func(req *Request, ctx *goproxy.ProxyCtx) bool {
		return strings.HasPrefix(req.RemoteAddr, ip+"")
	})
}

/*
* 某个IP是否在活动IP列表中间,如果是的话,需要特别地做个处理,将所有信息记录下来;
 */
func SrcIpWanted(redisConfig string) goproxy.ReqCondition {
	return goproxy.ReqConditionFunc(func(req *Request, ctx *goproxy.ProxyCtx) bool {
		ip := strings.Split(req.RemoteAddr, ":")[0]
		fmt.Println("got client from :", req.RemoteAddr)
		return IsActiveClients(redisConfig, ip)
	})
}

/**
* 运行代理服务器
 */
func RunProxy(hostAndPort string) {
	proxy := goproxy.NewProxyHttpServer()
	req_chan := make(chan *Identifier, 10000)
	//rep_chan := make(chan Response, 100000)
	go holdRedis(req_chan, "127.0.0.1:6379")
	//go fetchQueue()
	//go HoldRedisContent(rep_chan)
	go RemoveActiveClients("127.0.0.1:6379")
	var counter uint64
	counter = 0
	proxy.OnRequest(goproxy.ReqHostIs("www.showmyip.com")).DoFunc(func(r *Request, ctx *goproxy.ProxyCtx) (*Request, *Response) {
		return nil, goproxy.NewResponse(r, goproxy.ContentTypeHtml, StatusUnauthorized, fmt.Sprintf("<!doctype html><html><head><title>showmyip</title></head><body>ip is:{%s}<body/></html>", r.RemoteAddr))
	})

	proxy.OnResponse(SrcIpWanted("127.0.0.1:6379")).DoFunc(func(resp *Response, ctx *goproxy.ProxyCtx) *Response {
		fmt.Println("response done :", ctx.Req.URL)
		counter++
		ip := strings.Split(ctx.Req.RemoteAddr, ":")[0]
		req_chan <- &Identifier{Url: ctx.Req.URL.String(), Sn: counter, Ip: ip}
		if resp == nil {
			return resp
		}

		//fmt.Println("got ",resp)
		resp.Body = &CountReadCloser{ctx.Req.URL.String(), resp.Body, 0, make([]byte, 64), counter, "127.0.0.1:6379"}

		rds, err := redis.DialTimeout("tcp", "127.0.0.1:6379", time.Duration(10)*time.Second)
		errHndlr(err)
		defer rds.Close()
		sn := strconv.FormatUint(counter, 36)
		//将当前请求的URL记录在redis里 key就等于 ulr-{id}
		rds.Cmd("SET", fmt.Sprintf("url-%s", sn), ctx.Req.URL.String())
		headers, _ := json.Marshal(resp.Header)
		rds.Cmd("SET", fmt.Sprintf("headers-%s", sn), string(headers))
		fmt.Println("redis kv set:", fmt.Sprintf("url-%s", sn), ctx.Req.URL.String())
		//rep_chan <- *ctx.Resp
		return resp
	})
	log.Fatal(ListenAndServe(hostAndPort, proxy))
}
