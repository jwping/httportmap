package server

import (
	"context"
	"encoding/base64"
	"httportmap-server/internal/config"
	"httportmap-server/sessions"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	ginsessions "github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/jwping/logger"
)

var (
	store      = cookie.NewStore([]byte("voiceads-sre"))
	httPortMap = config.HttPortMap{
		PortMap: map[string]*config.DestAddr{},
	}
	mlog *logger.Logger
	once = sync.Once{}
)

func newAuthorizationHandle(srcPort string) func(w http.ResponseWriter, req *http.Request) {
	authorizationHandle := func(w http.ResponseWriter, req *http.Request) {
		if httPortMap.PortMap[srcPort].EnableAuth {
			s := sessions.NewSession("sre-session", req, store, w)
			session_ttl := s.Get("session_ttl_" + srcPort)
			ttl_unix, ok := session_ttl.(int64)

			if !ok || ttl_unix == 0 || time.Unix(ttl_unix, 0).Before(time.Now().Add(time.Hour*time.Duration(httPortMap.System.SessionTTL)*-1)) {
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

					// Authorization好像是删不掉的，是在一次浏览器生命周期中？
					// req.Header.Del("Authorization")
					// w.Header().Del("Authorization")
					s.Set("session_ttl_"+srcPort, time.Now().Unix())
					s.Save()

				default:
					w.Header().Set("WWW-Authenticate", `Basic realm="agent User Login"`)
					w.WriteHeader(http.StatusUnauthorized)
					return
				}

				// 因为我们加了身份验证头，所以在"向后传递"的时候需求去掉这个请求头（如果用户访问的后端本身也带这个请求头，，，那就GG）
				// req.Header.Del("Authorization")
			}
		}

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
			mlog.Debug("RoundTrip错误", err)
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

func startHttpServer(srcPort string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", newAuthorizationHandle(srcPort))
	mlog.Debug("linsten", srcPort, "destAddress", httPortMap.PortMap[srcPort].Dest)

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
	httPortMap.PortMap[srcPort].ShutdownCancel = cancel
	go func() {
		<-ctx.Done()
		srv.Shutdown(ctx)
	}()
}

func ReloadRun(hpm config.HttPortMap, mlogger *logger.Logger) {
	once.Do(func() {
		mlog = mlogger
	})

	store.Options(ginsessions.Options{MaxAge: hpm.System.SessionTTL * 60 * 60})

	for srcPort, destAddr := range httPortMap.PortMap {
		_, ok := hpm.PortMap[srcPort]
		if !ok {
			mlog.Debug("从映射池中移除", srcPort)
			destAddr.ShutdownCancel()
			delete(httPortMap.PortMap, srcPort)
		}
	}

	for srcPort, destAddr := range hpm.PortMap {
		destAddr.SrcPort = srcPort

		_, ok := httPortMap.PortMap[srcPort]

		if ok {
			httPortMap.PortMap[srcPort].Update(*destAddr)
		} else {
			httPortMap.PortMap[srcPort] = destAddr
			startHttpServer(srcPort)
		}
	}

	httPortMap.System = hpm.System
}
