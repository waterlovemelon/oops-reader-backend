# 预解析阅读产物部署说明

预处理程序与 API 使用同一份 catalog 根目录。程序不会在阅读请求中打开 EPUB；每本书的产物位于 `<epub 所在目录>/.reading/<book-id>/<version>/`，`current` 是指向已完成版本的符号链接。

构建服务器 amd64 二进制（源码位于 `oops-reader-app/packages/reader_content/bin/reader_content_preprocess.dart`）：

```sh
cd oops-reader-app
dart compile exe packages/reader_content/bin/reader_content_preprocess.dart \
  -o /tmp/reader_content_preprocess
```

补建单本书时使用书籍文件的父目录作为 `--output`，先写临时目录，成功后才发布：

```sh
/opt/oops-reader/bin/reader_content_preprocess \
  --source /opt/oops-reader/catalog/originals/epub/3b/f4/book.epub \
  --output /opt/oops-reader/catalog \
  --book-id book-id
```

参数只有双横线形式（`--source` / `--output` / `--book-id`），单横线会打印 usage 并以退出码 2 结束。正常导入由 manager 的导入流程自动调用（`catalog.reading_preprocess` 配置），只有在补建历史书籍或修复失败产物时才需要手工执行。

导入 worker 应为每本书串行或设置有限并发；不要把整个书库同时读入内存。失败时保留旧的 `current` 版本，修复后可重复执行。删除旧版本前必须确认没有读请求引用它，并至少保留当前版本及迁移期版本。程序、配置和数据库迁移由运维按备份与回滚流程发布。
