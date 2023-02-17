# httportmap-server
> A multi-address port mapping tool with authentication
> It can be described by various names, such as port mapping tool, proxy tool, address pool proxy, address pool mapping, multiple back-end identity proxy, and so on


```shell
$ go run main.go --help
Usage of httportmap-server:
  -config string
        指定配置文件 (default "config.yaml")
  -ifs string
        指定http字典每行的分隔符 (default ":")
  -log_level int
        指定日志输出等级
                LevelDebug Level = -4
                LevelInfo  Level = 0
                LevelWarn  Level = 4
                LevelError Level = 8
  -map string
        指定http端口字典，用来生成配置文件的，指定此项之后只会生成配置文件！（没有默认值，不指定则使用配置文件来启动服务）

# 通过 http_map 生成配置文件
$ go run main.go -map http_map
{"time":"2023-02-17T11:09:52.8124226+08:00","level":"INFO","source":"/home/jwping/gopath/httportmap-server/main.go:56","msg":"配置文件生成中..."}
{"time":"2023-02-17T11:09:52.8127413+08:00","level":"INFO","source":"/home/jwping/gopath/httportmap-server/internal/generate/generate.go:77","msg":"输出文件路径","!BADKEY":"config_1676603392.yaml"}
{"time":"2023-02-17T11:09:52.8127847+08:00","level":"INFO","source":"/home/jwping/gopath/httportmap-server/main.go:63","msg":"生成成功！"}

# 运行，注意：默认是使用config.yaml配置文件
$ go run main.go
{"time":"2023-02-17T10:58:02.8313642+08:00","level":"INFO","source":"/home/jwping/gopath/httportmap-server/internal/server/server.go:27","msg":"服务管理接口已启动","!BADKEY":":8080"}
{"time":"2023-02-17T10:58:02.8315939+08:00","level":"INFO","source":"/home/jwping/gopath/httportmap-server/main.go:89","msg":"服务启动完成！"}
```