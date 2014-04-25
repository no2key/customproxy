package proxy

import (
	"fmt"
	//"github.com/astaxie/beego"
	"github.com/astaxie/beego/context"
	"github.com/elazarl/goproxy"
	//"github.com/elazarl/goproxy/ext/html"
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
			fmt.Println("lpush:", cnt.Ip, strconv.FormatUint(cnt.Sn, 36))
			fmt.Println("push in list:", cnt.Url)

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
 */
func PushActiveClient(redisConfig string, ctx *context.Context) {
	ip := strings.Split(ctx.Request.RemoteAddr, ":")[0]
	rds, err := redis.DialTimeout("tcp", redisConfig, time.Duration(10)*time.Second)
	errHndlr(err)
	defer rds.Close()
	rds.Cmd("SADD", "activeIPs", ip)
	//fmt.Println("sadd members:", s)
}

func fetchQueue() {
	rds, err := redis.DialTimeout("tcp", "127.0.0.1:6379", time.Duration(10)*time.Second)
	errHndlr(err)
	defer rds.Close()
	for {
		time.Sleep(time.Second * 1)
		s, _ := rds.Cmd("rpop", "127.0.0.1").Str()
		fmt.Println("fetched from Redis:", s)
	}
}
func FetchUrlList4Ip(redisConfig string, ip string) []map[string]interface{} {
	rds, err := redis.DialTimeout("tcp", redisConfig, time.Duration(10)*time.Second)
	errHndlr(err)
	defer rds.Close()
	s, _ := rds.Cmd("LRANGE", ip, 0, 100).List()

	//fmt.Println(...)
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
func SrcIpBeginWith(ip string) goproxy.ReqCondition {
	return goproxy.ReqConditionFunc(func(req *Request, ctx *goproxy.ProxyCtx) bool {
		return strings.HasPrefix(req.RemoteAddr, ip+"")
	})
}
func SrcIpWanted(redisConfig string) goproxy.ReqCondition {
	return goproxy.ReqConditionFunc(func(req *Request, ctx *goproxy.ProxyCtx) bool {
		ip := strings.Split(req.RemoteAddr, ":")[0]
		return IsActiveClients(redisConfig, ip)
	})
}
func RunProxy() {
	proxy := goproxy.NewProxyHttpServer()

	req_chan := make(chan *Identifier, 10000)
	//rep_chan := make(chan Response, 100000)
	go holdRedis(req_chan, "127.0.0.1:6379")
	//go fetchQueue()
	//go HoldRedisContent(rep_chan)
	var counter uint64
	counter = 0
	proxy.OnResponse(SrcIpWanted("127.0.0.1:6379")).DoFunc(func(resp *Response, ctx *goproxy.ProxyCtx) *Response {
		fmt.Println("response done :", ctx.Req.URL)
		counter++
		ip := strings.Split(ctx.Req.RemoteAddr, ":")[0]
		req_chan <- &Identifier{Url: ctx.Req.URL.String(), Sn: counter, Ip: ip}
		if resp == nil {
			return resp
		}
		resp.Body = &CountReadCloser{ctx.Req.URL.String(), resp.Body, 0, make([]byte, 64), counter, "127.0.0.1:6379"}

		rds, err := redis.DialTimeout("tcp", "127.0.0.1:6379", time.Duration(10)*time.Second)
		errHndlr(err)
		defer rds.Close()
		sn := strconv.FormatUint(counter, 36)
		//将当前请求的URL记录在redis里 key就等于 ulr-{id}
		rds.Cmd("SET", fmt.Sprintf("url-%s", sn), ctx.Req.URL.String())
		fmt.Println("redis kv set:", fmt.Sprintf("url-%s", sn), ctx.Req.URL.String())
		//rep_chan <- *ctx.Resp
		return resp
	})
	log.Fatal(ListenAndServe(":8080", proxy))
}
