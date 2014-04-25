package proxy

import (
	"fmt"
	"github.com/astaxie/beego/context"
	"github.com/elazarl/goproxy"
	//"github.com/elazarl/goproxy/ext/html"
	"github.com/fzzy/radix/redis"
	"io"
	"log"
	. "net/http"
	"os"
	"strings"
	"time"
)

type Count struct {
	Id    string
	Count int64
}
type CountReadCloser struct {
	Id string
	R  io.ReadCloser
	ch chan<- Count
	nr int64
}

func (c *CountReadCloser) Read(b []byte) (n int, err error) {
	n, err = c.R.Read(b)
	c.nr += int64(n)
	return
}
func (c CountReadCloser) Close() error {
	c.ch <- Count{c.Id, c.nr}
	return c.R.Close()
}

func errHndlr(err error) {
	if err != nil {
		fmt.Println("error:", err)
		os.Exit(1)
	}
}
func holdRedis(req_chan chan Request) {
	rds, err := redis.DialTimeout("tcp", "127.0.0.1:6379", time.Duration(10)*time.Second)
	errHndlr(err)
	defer rds.Close()
	for {
		select {
		case req := <-req_chan:
			s := rds.Cmd("lpush", "127.0.0.1", req.URL)
			fmt.Println("push in list:", req.URL)
			if s == nil {

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
func HoldRedisContent(rep_chan chan Response) {
	rds, err := redis.DialTimeout("tcp", "127.0.0.1:6379", time.Duration(10)*time.Second)
	errHndlr(err)
	defer rds.Close()

	for {
		select {
		case res := <-rep_chan:
			fmt.Println("got new resp msg:")
			var b []byte
			res.Body.Read(b)
			var st string
			a, _ := res.Body.Read(b)
			st = string(a)
			rds.Cmd("set", res.Request.URL, st).Str()

			fmt.Println("read bytes:", st)
		}
	}
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
func FetchUrlList4Ip(redisConfig string, ip string) []string {
	rds, err := redis.DialTimeout("tcp", redisConfig, time.Duration(10)*time.Second)
	errHndlr(err)
	defer rds.Close()
	s, _ := rds.Cmd("LRANGE", ip, 0, 100).List()
	return s
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

	req_chan := make(chan Request, 10000)
	rep_chan := make(chan Response, 100000)
	go holdRedis(req_chan)
	//go fetchQueue()
	go HoldRedisContent(rep_chan)
	// proxy.OnRequest().DoFunc(func(r *Request, ctx *goproxy.ProxyCtx) *Request {
	// 	fmt.Println(ctx.Req.Host, ctx.Req.Header, ctx.Req.URL, ctx.Req.RemoteAddr)
	// 	return r
	// })

	//**
	// var IsLocalHost ReqConditionFunc = func(req *http.Request, ctx *ProxyCtx) bool {
	// 	return req.URL.Host == "::1" ||
	// 		req.URL.Host == "0:0:0:0:0:0:0:1" ||
	// 		localHostIpv4.MatchString(req.URL.Host) ||
	// 		req.URL.Host == "localhost"
	// }
	proxy.OnRequest(SrcIpWanted("127.0.0.1:6379")).DoFunc(func(r *Request, ctx *goproxy.ProxyCtx) (*Request, *Response) {
		req_chan <- *r
		return r, ctx.Resp
	})
	// proxy.OnResponse(goproxy_html.IsWebRelatedText).DoFunc(func(resp *Response, ctx *goproxy.ProxyCtx) *Response {
	// 	fmt.Println("response done :", ctx.Req.URL)
	// 	resp.Body = &CountReadCloser{ctx.Req.URL.String(), resp.Body, nil, 0}
	// 	//rep_chan <- *ctx.Resp
	// 	return resp
	// })
	log.Fatal(ListenAndServe(":8080", proxy))
}
