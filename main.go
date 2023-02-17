package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"httportmap-server/internal/config"
	"httportmap-server/internal/generate"
	"httportmap-server/internal/logger"
	"httportmap-server/internal/server"
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

func main() {
	cPath := flag.String("config", "config.yaml", "指定配置文件")
	mPath := flag.String("map", "", "指定http端口字典，用来生成配置文件的，指定此项之后只会生成配置文件！（没有默认值，不指定则使用配置文件来启动服务）")
	ifs := flag.String("ifs", ":", "指定http字典每行的分隔符")
	logLevel := flag.Int("log_level", 0, `指定日志输出等级
	LevelDebug Level = -4
	LevelInfo  Level = 0
	LevelWarn  Level = 4
	LevelError Level = 8`)
	flag.Parse()

	hlog := logger.NewLogger(*logLevel)

	if *mPath != "" {
		hlog.Info("配置文件生成中...")
		err := generate.GenerateConfig(*mPath, *ifs, hlog)
		if err != nil {
			hlog.Info("配置生成失败，请检查！")
			return
		}

		hlog.Info("生成成功！")
		return
	}

	reload := func() error {
		httpPortMap, err := config.ReadConfig(*cPath, hlog)
		if err != nil {
			hlog.Info("加载配置文件失败，请检查！")
			return err
		}

		server.ReloadRun(*httpPortMap, hlog)
		return nil
	}

	err := reload()
	if err != nil {
		hlog.Info("下班...")
		return
	}

	server.HttpRun(hlog)

	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGHUP)

	hlog.Info("服务启动完成！")

	for {
		<-c
		reload()
	}
}
