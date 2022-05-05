package main

import (
	"encoding/base64"
	"flag"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"strings"

	"github.com/elazarl/goproxy"
	"github.com/elazarl/goproxy/ext/auth"
	"gopkg.in/yaml.v3"
)

// http_proxy 不是我们想要的
func main2() {
	proxy := goproxy.NewProxyHttpServer()
	proxy.Verbose = true
	auth.ProxyBasic(proxy, "test123", func(user, passwd string) bool {
		log.Printf("验证的用户：%s - %s\n", user, passwd)
		if user != "anshan" || passwd != "hello" {
			return false
		}
		return true
	})

	// https也一样过滤， 代价就是ssl证书需要手动提供
	proxy.OnRequest().HandleConnect(goproxy.AlwaysMitm)

	proxy.OnRequest().DoFunc(func(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
		ip, _, err := net.SplitHostPort(req.RemoteAddr)
		if err != nil {
			log.Print(err)
		}
		log.Printf("[%d] %s --> %s %s", ctx.Session, ip, req.Method, req.URL)
		return req, nil
	})

	log.Fatal(http.ListenAndServe(":8080", proxy))
}

func newAuthorizationHandle(destAddr DestAddr) func(w http.ResponseWriter, req *http.Request) {
	authorizationHandle := func(w http.ResponseWriter, req *http.Request) {
		auth := req.Header.Get("Authorization")
		if auth == "" {
			w.Header().Set("WWW-Authenticate", `Basic realm="Dotcoo User Login"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		auths := strings.SplitN(auth, " ", 2)
		if len(auths) != 2 {
			w.Header().Set("WWW-Authenticate", `Basic realm="agent User Login"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		authMethod := auths[0]
		authB64 := auths[1]
		switch authMethod {
		case "Basic":
			authstr, err := base64.StdEncoding.DecodeString(authB64)
			if err != nil {
				w.Header().Set("WWW-Authenticate", `Basic realm="agent User Login"`)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			userPwd := strings.SplitN(string(authstr), ":", 2)
			if len(userPwd) != 2 {
				w.Header().Set("WWW-Authenticate", `Basic realm="agent User Login"`)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			username := userPwd[0]
			password := userPwd[1]
			if destAddr.Authorization[username] != password {
				w.Header().Set("WWW-Authenticate", `Basic realm="agent User Login"`)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		default:
			w.Header().Set("WWW-Authenticate", `Basic realm="agent User Login"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		// io.WriteString(w, "hello, world!\n")

		// fmt.Printf("请求头：%v\n", req.Header)
		transport := http.DefaultTransport
		outReq := new(http.Request)
		*outReq = *req // this only does shallow copies of maps
		// 因为我们加了身份验证头，所以在"向后传递"的时候需求去掉这个请求头（如果用户访问的后端本身也带这个请求头，，，那就GG）
		outReq.Header.Del("Authorization")
		outReq.URL.Scheme = "http"
		outReq.URL.Host = destAddr.Dest
		if clientIP, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
			if prior, ok := outReq.Header["X-Forwarded-For"]; ok {
				clientIP = strings.Join(prior, ", ") + ", " + clientIP
			}
			outReq.Header.Set("X-Forwarded-For", clientIP)
		}

		res, err := transport.RoundTrip(outReq)
		if err != nil {
			log.Printf("RoundTrip错误: %v\n", err)
			w.WriteHeader(http.StatusBadGateway)
			return
		}

		for key, value := range res.Header {
			for _, v := range value {
				w.Header().Add(key, v)
			}
		}
		w.WriteHeader(res.StatusCode)
		io.Copy(w, res.Body)
		res.Body.Close()
	}

	return authorizationHandle
}

type DestAddr struct {
	Authorization map[string]string `yaml:"authorization"`
	Dest          string            `yaml:"dest"`
}

type ProxyPortMap struct {
	PortMap map[string]DestAddr `yaml:"portmap"`
}

func startHttpServer(srcPort string, destAddr DestAddr) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", newAuthorizationHandle(destAddr))
	log.Printf("linsten %s -> %s\n", srcPort, destAddr.Dest)
	err := http.ListenAndServe(":"+srcPort, mux)
	if err != nil {
		log.Printf("%s listen err: %v\n", srcPort, err)
	}
}

func main() {
	cPath := flag.String("config", "config.yaml", "指定配置文件")
	flag.Parse()
	cData, err := ioutil.ReadFile(*cPath)
	if err != nil {
		log.Printf("读取配置文件异常：%v\n", err)
		return
	}
	var proxyPortMap ProxyPortMap
	yaml.Unmarshal(cData, &proxyPortMap)

	for srcPort, destAddr := range proxyPortMap.PortMap {
		go startHttpServer(srcPort, destAddr)
	}

	select {}
}
