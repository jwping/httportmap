package server

import (
	"encoding/json"
	"net/http"

	"github.com/jwping/logger"
)

func HttpRun(logger *logger.Logger) {
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

	go http.ListenAndServe(httPortMap.System.Listen, nil)
	logger.Info("服务管理接口已启动", httPortMap.System.Listen)
}
