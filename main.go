package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	gonanoid "github.com/matoous/go-nanoid/v2"
	"gopkg.in/yaml.v3"
)

// http_proxy 不是我们想要的
// func main2() {
// 	proxy := goproxy.NewProxyHttpServer()
// 	proxy.Verbose = true
// 	auth.ProxyBasic(proxy, "test123", func(user, passwd string) bool {
// 		log.Printf("验证的用户：%s - %s\n", user, passwd)
// 		if user != "anshan" || passwd != "hello" {
// 			return false
// 		}
// 		return true
// 	})

// 	// https也一样过滤， 代价就是ssl证书需要手动提供
// 	proxy.OnRequest().HandleConnect(goproxy.AlwaysMitm)

// 	proxy.OnRequest().DoFunc(func(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
// 		ip, _, err := net.SplitHostPort(req.RemoteAddr)
// 		if err != nil {
// 			log.Print(err)
// 		}
// 		log.Printf("[%d] %s --> %s %s", ctx.Session, ip, req.Method, req.URL)
// 		return req, nil
// 	})

// 	log.Fatal(http.ListenAndServe(":8080", proxy))
// }

func newAuthorizationHandle(srcPort string) func(w http.ResponseWriter, req *http.Request) {
	authorizationHandle := func(w http.ResponseWriter, req *http.Request) {
		if httPortMap.PortMap[srcPort].EnableAuth {
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
				if err != nil || string(authstr) == ":" {
					w.Header().Set("WWW-Authenticate", `Basic realm="agent User Login"`)
					w.WriteHeader(http.StatusUnauthorized)
					return
				}

				userPwd := strings.SplitN(string(authstr), ":", 2)
				if len(userPwd) != 2 || userPwd[0] == "" || userPwd[1] == "" {
					w.Header().Set("WWW-Authenticate", `Basic realm="agent User Login"`)
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				username := userPwd[0]
				password := userPwd[1]
				if httPortMap.PortMap[srcPort].Authorization[username] != password {
					w.Header().Set("WWW-Authenticate", `Basic realm="agent User Login"`)
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
			default:
				w.Header().Set("WWW-Authenticate", `Basic realm="agent User Login"`)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			// 因为我们加了身份验证头，所以在"向后传递"的时候需求去掉这个请求头（如果用户访问的后端本身也带这个请求头，，，那就GG）
			req.Header.Del("Authorization")
		}

		// io.WriteString(w, "hello, world!\n")

		transport := http.DefaultTransport
		outReq := new(http.Request)
		*outReq = *req // this only does shallow copies of maps
		outReq.URL.Scheme = "http"
		outReq.URL.Host = httPortMap.PortMap[srcPort].Dest
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
	// Enable        bool              `yaml:"enable"`
	EnableAuth    bool              `yaml:"enable_auth"`
	Authorization map[string]string `yaml:"authorization"`
	Dest          string            `yaml:"dest"`

	srcPort string
	// server  *http.Server
	shutdownCancel context.CancelFunc
}

func (d *DestAddr) update(dest DestAddr) {
	d.srcPort = dest.srcPort
	d.EnableAuth = dest.EnableAuth
	d.Authorization = dest.Authorization
	d.Dest = dest.Dest
	// d.Enable = dest.Enable

}

type System struct {
	Listen string `yaml:"listen"`
}

type HttPortMap struct {
	System  System               `yaml:"system"`
	PortMap map[string]*DestAddr `yaml:"portmap"`
}

func startHttpServer(srcPort string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", newAuthorizationHandle(srcPort))
	log.Printf("linsten %s -> %s\n", srcPort, httPortMap.PortMap[srcPort].Dest)

	// listener, err := net.Listen("tcp", ":"+srcPort)
	// if err != nil {
	// 	log.Printf("tcp:%s listener err: %v\n", srcPort, err)
	// 	return
	// }

	// destAddr := proxyPortMap.PortMap[srcPort]
	// destAddr.listener = listener

	srv := http.Server{
		Addr:    ":" + srcPort,
		Handler: mux,
	}

	go srv.ListenAndServe()

	ctx, cancel := context.WithCancel(context.Background())
	httPortMap.PortMap[srcPort].shutdownCancel = cancel
	go func() {
		select {
		case <-ctx.Done():
			srv.Shutdown(ctx)
		}
	}()
}

var httPortMap = HttPortMap{
	PortMap: map[string]*DestAddr{},
}

func reloadConfig(cPath string) {
	cData, err := ioutil.ReadFile(cPath)
	if err != nil {
		log.Printf("读取配置文件异常：%v\n", err)
		return
	}

	var thttPortMap HttPortMap
	if err := yaml.Unmarshal(cData, &thttPortMap); err != nil {
		log.Printf("yaml 反序列化失败：%v\n", err)
		return
	}

	// fmt.Printf("反序列化结构：%v\n", tproxyPortMap.PortMap["38085"])

	for srcPort, destAddr := range httPortMap.PortMap {
		_, ok := thttPortMap.PortMap[srcPort]
		if !ok {
			log.Printf("%s 从映射地址池中移除！\n", srcPort)
			destAddr.shutdownCancel()
			delete(httPortMap.PortMap, srcPort)
		}
	}

	for srcPort, destAddr := range thttPortMap.PortMap {
		destAddr.srcPort = srcPort

		_, ok := httPortMap.PortMap[srcPort]

		if ok {
			httPortMap.PortMap[srcPort].update(*destAddr)
		} else {
			httPortMap.PortMap[srcPort] = destAddr
			startHttpServer(srcPort)
		}
	}

	httPortMap.System = thttPortMap.System
}

func httpServer() {
	http.HandleFunc("/map", func(w http.ResponseWriter, r *http.Request) {

		portMap := map[string]string{}
		for srcPort, destAddr := range httPortMap.PortMap {
			portMap[srcPort] = destAddr.Dest
		}

		data, err := json.Marshal(portMap)
		if err != nil {
			w.Write([]byte(err.Error()))
		}

		w.Write(data)
	})

	log.Printf("system listen: %s\n", httPortMap.System.Listen)
	go http.ListenAndServe(httPortMap.System.Listen, nil)
}

func generateConfig(mPath, ifs string) {
	data, err := ioutil.ReadFile(mPath)
	if err != nil {
		log.Panicf("ReadFile err: %v\n", err)
	}

	portMap := map[string]*DestAddr{}

	buffer := bytes.NewBuffer(data)
	for {
		line, err := buffer.ReadString(byte('\n'))
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Printf("ReadString err: %v\n", err)
			return
		}

		line = strings.Replace(line, "\n", "", -1)

		array := strings.Split(line, ifs)

		nanoid, err := gonanoid.New()
		if err != nil {
			log.Printf("%s nanoid生成失败：%v\n", array[0], err)
			continue
		}
		portMap[array[0]] = &DestAddr{
			EnableAuth: true,
			Authorization: map[string]string{
				"ops": nanoid,
			},
			Dest: array[1] + ":" + array[2],
		}

	}

	httPortMap := HttPortMap{
		System: System{
			Listen: ":8080",
		},
		PortMap: portMap,
	}

	outData, err := yaml.Marshal(httPortMap)
	if err != nil {
		log.Printf("Marshal err: %v\n", err)
		return
	}

	if err = ioutil.WriteFile(fmt.Sprintf("config_%d.yaml", time.Now().Unix()), outData, 0660); err != nil {
		log.Printf("WriteFile err: %v\n", err)
		return
	}

}

func main() {
	cPath := flag.String("config", "config.yaml", "指定配置文件")
	mPath := flag.String("map", "", "指定http端口字典，用来生成配置文件的，指定此项之后只会生成配置文件！（没有默认值，不指定则使用配置文件来启动服务）")
	ifs := flag.String("ifs", ":", "指定http字典每行的分隔符")
	flag.Parse()

	if *mPath != "" {
		log.Printf("生成配置文件！\n")
		generateConfig(*mPath, *ifs)
		return
	}

	reloadConfig(*cPath)
	httpServer()

	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGHUP)

	for {
		select {
		case <-c:
			reloadConfig(*cPath)
		}
	}
}
