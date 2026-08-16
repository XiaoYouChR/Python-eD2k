# goed2k

`goed2k` 是一个用 Go 编写的 ED2K/eMule 客户端库，附带一个可交互的终端下载管理器。

![Demo](.docs/demo.gif)

## AI 参与开发

这个项目主要由 AI 辅助完成，使用的工具包括：

- `Codex`
- `ChatGPT 5.4`

实现过程中主要参考了仓库内的两个开源项目：

- `jed2k`
- `amule`

## 特性

`goed2k` 目前已经覆盖了一套可用的 ED2K 客户端基础能力，主要包括：

- [x] ED2K 文件下载
- [x] 多任务并发下载
- [x] 多个 ED2K server 并发找源
- [x] `server.met` 加载
- [x] KAD bootstrap 和 source 查找
- [x] UPNP 端口映射
- [x] 资源搜索
- [x] 暂停、继续、删除任务
- [x] 状态持久化与恢复
- [x] 上传支持
- [x] 任务、peer、server、piece 状态快照
- [x] 任务进度订阅
- [x] 可交互的终端下载管理器

## 安装

### 可执行文件

```bash
go install github.com/monkeyWie/goed2k/cmd/goed2k@latest
```

### 作为库

```bash
go get github.com/monkeyWie/goed2k
```

## 快速开始

### 运行终端下载管理器

```bash
goed2k
```

如果你想直接从源码运行：

```bash
go run ./cmd/goed2k
```

## 库使用示例

```go
package main

import (
	"log"

	"github.com/monkeyWie/goed2k"
)

func main() {
	settings := goed2k.NewSettings()
	settings.ReconnectToServer = true
	settings.EnableUPnP = true

	client := goed2k.NewClient(settings)
	if err := client.Start(); err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	if err := client.ConnectServers("176.123.5.89:4725", "45.82.80.155:5687"); err != nil {
		log.Fatal(err)
	}

	if _, _, err := client.AddLink(
		"ed2k://|file|example-file.mp3|12345678|0123456789ABCDEF0123456789ABCDEF|/",
		"./downloads",
	); err != nil {
		log.Fatal(err)
	}

	if err := client.Wait(); err != nil && err != goed2k.ErrClientStopped {
		log.Fatal(err)
	}
}
```

## License

本项目采用 MIT License。

你可以在保留原始版权声明和许可声明的前提下，自由使用、复制、修改、合并、发布、分发、再许可和销售本项目的副本。项目按“现状”提供，作者不对其适用性或稳定性作任何明示或暗示担保。
