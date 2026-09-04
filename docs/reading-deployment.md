# 预解析阅读产物部署说明

预处理程序与 API 使用同一份 catalog 根目录。程序不会在阅读请求中打开 EPUB；每本书的产物位于 `<epub 所在目录>/.reading/<book-id>/<version>/`，`current` 是指向已完成版本的符号链接。

构建服务器 amd64 二进制：

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o reading-preprocess ./cmd/reading-preprocess
```

补建单本书时使用书籍文件的父目录作为 `-output`，先写临时目录，成功后才发布：

```sh
./reading-preprocess -source /opt/oops-reader/catalog/book.epub \
  -output /opt/oops-reader/catalog -book-id book-id
```

导入 worker 应为每本书串行或设置有限并发；不要把整个书库同时读入内存。失败时保留旧的 `current` 版本，修复后可重复执行。删除旧版本前必须确认没有读请求引用它，并至少保留当前版本及迁移期版本。程序、配置和数据库迁移由运维按备份与回滚流程发布。
