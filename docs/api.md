# Oops Reader Backend API 文档

> 版本: v1.0.0
> 更新日期: 2025-03-16
> 基础URL: `http://localhost:8080`

---

## 目录

- [概述](#概述)
- [通用说明](#通用说明)
- [接口列表](#接口列表)
- [接口详情](#接口详情)
- [错误码说明](#错误码说明)
- [开发工具](#开发工具)

---

## 概述

Oops Reader Backend 是为 Oops Reader 提供的后端服务，主要提供以下功能：
- 书籍信息解析与检索
- 书籍封面获取
- 健康检查

### 技术栈
- **语言**: Go 1.26.1
- **框架**: Go 标准库 HTTP
- **数据处理**: Python 3.11+
- **数据源**: 豆瓣、Google Books、百度图片

---

## 通用说明

### 请求格式
- Content-Type: `application/json`
- 请求体编码: `UTF-8`

### 响应格式
所有响应均为 JSON 格式，字符编码为 `UTF-8`。

#### 成功响应示例
```json
{
  "status": "ok",
  "data": {
    // 数据内容
  }
}
```

#### 错误响应示例
```json
{
  "error": "错误类型",
  "message": "错误详情",
  "details": "详细说明（可选）"
}
```

### HTTP 状态码
| 状态码 | 说明 |
|--------|------|
| 200 | 请求成功 |
| 400 | 请求参数错误 |
| 404 | 资源不存在 |
| 405 | 请求方法不允许 |
| 500 | 服务器内部错误 |

---

## 接口列表

| 接口名称 | 方法 | 路径 | 描述 |
|---------|------|------|------|
| 健康检查 | GET | `/health` | 检查服务健康状态 |
| 书籍解析 | POST | `/v1/utils/parse-book-info` | 解析书籍名称和作者并检索信息 |
| 书籍封面 | GET | `/v1/utils/book-cover` | 获取书籍封面图片 |

---

## 接口详情

### 1. 健康检查

检查服务是否正常运行。

#### 请求
```http
GET /health HTTP/1.1
Host: localhost:8080
```

#### 响应
**状态码**: 200

```json
{
  "message": "service is healthy",
  "status": "ok"
}
```

#### 字段说明
| 字段 | 类型 | 说明 |
|------|------|------|
| message | string | 状态消息 |
| status | string | 状态标识，固定为 "ok" |

#### 示例（curl）
```bash
curl http://localhost:8080/health
```

---

### 2. 书籍解析

解析输入字符串中的书籍名称和作者信息，并联网检索详细的书籍元数据。

#### 请求
```http
POST /v1/utils/parse-book-info HTTP/1.1
Host: localhost:8080
Content-Type: application/json

{
  "input": "三体"
}
```

#### 请求参数

**Body 参数:**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| input | string | 是 | 包含书籍名称和作者的字符串 |

**input 格式支持:**
- 仅书名：`"三体"`
- 书名+作者（斜杠分隔）：`"1984/乔治·奥威尔"`
- 书名+作者（其他格式）：`"三体 刘慈欣"`

#### 响应

**状态码**: 200

```json
{
  "input": "三体",
  "parsed_title": "三体",
  "parsed_author": null,
  "search_results": [
    {
      "source": "douban",
      "title": "三体",
      "author": null,
      "found": false
    },
    {
      "source": "google_books",
      "title": "Trekropparsproblemet",
      "author": "Liu Cixin",
      "isbn": null,
      "url": "https://play.google.com/store/books/details?id=h9L3EAAAQBAJ&source=gbs_api",
      "cover": "http://books.google.com/books/content?id=h9L3EAAAQBAJ&printsec=frontcover&img=1&zoom=1&edge=curl&source=gbs_api",
      "rating": null,
      "found": true
    }
  ],
  "best_match": {
    "source": "google_books",
    "title": "Trekropparsproblemet",
    "author": "Liu Cixin",
    "isbn": null,
    "url": "https://play.google.com/store/books/details?id=h9L3EAAAQBAJ&source=gbs_api",
    "cover": "http://books.google.com/books/content?id=h9L3EAAAQBAJ&printsec=frontcover&img=1&zoom=1&edge=curl&source=gbs_api",
    "rating": null,
    "found": true
  }
}
```

#### 响应字段说明

**顶层字段:**

| 字段 | 类型 | 说明 |
|------|------|------|
| input | string | 原始输入字符串 |
| parsed_title | string | 解析出的书名 |
| parsed_author | string|null | 解析出的作者名 |
| search_results | array | 所有搜索结果数组 |
| best_match | object|null | 最佳匹配项（来自search_results） |

**search_results 数组元素:**

| 字段 | 类型 | 说明 |
|------|------|------|
| source | string | 数据源（豆瓣/google_books） |
| title | string | 书名 |
| author | string|null | 作者名 |
| isbn | string|null | ISBN号 |
| url | string|null | 书籍详情URL |
| cover | string|null | 封面图片URL |
| rating | number|null | 评分（0-5） |
| found | boolean | 是否找到匹配 |

#### 错误响应

**请求体格式错误 (400)**
```json
{
  "error": "Invalid request body",
  "details": "unexpected end of JSON input"
}
```

**解析失败 (500)**
```json
{
  "error": "Failed to parse book info",
  "details": "...错误详情...",
  "output": "Python脚本输出或错误信息"
}
```

#### 示例

**示例 1: 仅书名（中文书籍）**
```bash
curl -X POST http://localhost:8080/v1/utils/parse-book-info \
  -H "Content-Type: application/json" \
  -d '{"input": "三体"}'
```

**示例 2: 书名+作者（斜杠分隔）**
```bash
curl -X POST http://localhost:8080/v1/utils/parse-book-info \
  -H "Content-Type: application/json" \
  -d '{"input": "1984/乔治·奥威尔"}'
```

**示例 3: 英文书籍**
```bash
curl -X POST http://localhost:8080/v1/utils/parse-book-info \
  -H "Content-Type: application/json" \
  -d '{"input": "Clean Code"}'
```

**示例 4: 带拼音**
```bash
curl -X POST http://localhost:8080/v1/utils/parse-book-info \
  -H "Content-Type: application/json" \
  -d '{"input": "San Ti / Liu Cixin"}'
```

#### 数据源优先级
1. **豆瓣** - 中文书籍首选
2. **Google Books** - 英文书籍/备选

#### 性能说明
- 平均响应时间: 7-15 秒
- 依赖外部服务，受网络影响
- 建议客户端实现超时控制（建议 30 秒）

---

### 3. 书籍封面

根据书籍名称获取封面图片URL。

#### 请求
```http
GET /v1/utils/book-cover?book_name=三体 HTTP/1.1
Host: localhost:8080
```

#### 请求参数

**Query 参数:**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| book_name | string | 是 | 书籍名称 |

#### 响应

**状态码**: 200

```json
{
  "source": "baidu-images",
  "title": "三体",
  "author": null,
  "imageUrl": "https://lib.gzccc.edu.cn/__local/5/DB/03/E42FDD1ACAF1A4C86CAD01A5414_8283B924_4B4BF.jpeg"
}
```

**或（控制台风格输出）：**

```
正在搜索: 三体

[水印检测] 已启用水印检测功能
[第一轮] 搜索无水印封面...
正在尝试: 豆瓣网页...
正在尝试: 百度图片...
✓ 成功从 百度图片 获取无水印封面
书名: 三体
来源: baidu-images
封面: https://lib.gzccc.edu.cn/__local/5/DB/03/E42FDD1ACAF1A4C86CAD01A5414_8283B924_4B4BF.jpeg
```

#### 响应字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| source | string | 图片来源 |
| title | string | 识别的书名 |
| author | string|null | 作者名（可选） |
| imageUrl | string | 封面图片URL |

#### 来源说明

| source | 说明 | 备注 |
|--------|------|------|
| baidu-images | 百度图片 | 默认来源，支持去水印 |
| douban | 豆瓣 | 备选来源 |
| google | Google Books | 备选来源 |
| none | 未找到 | 无可用封面 |

#### 错误响应

**缺少参数 (400)**
```json
{
  "error": "book_name query parameter is required"
}
```

**获取失败 (500)**
```json
{
  "error": "Failed to get book cover",
  "details": "...错误详情...",
  "output": "Python脚本输出或错误信息"
}
```

#### 示例

**示例 1: 中文书籍**
```bash
curl "http://localhost:8080/v1/utils/book-cover?book_name=三体"
```

**示例 2: 英文书籍**
```bash
curl "http://localhost:8080/v1/utils/book-cover?book_name=Clean%20Code"
```

**示例 3: 使用 ISBN**
```bash
curl "http://localhost:8080/v1/utils/book-cover?book_name=9787536692930"
```

#### 特性说明
- **自动去水印**: 图片会经过水印检测和处理
- **多源搜索**: 按优先级搜索多个图片源
- **质量优化**: 优先选择高质量封面

#### 性能说明
- 平均响应时间: 5-10 秒
- 依赖外部服务，受网络影响
- 建议客户端实现超时控制（建议 20 秒）

---

## 错误码说明

### HTTP 状态码

| 状态码 | 说明 | 处理建议 |
|--------|------|---------|
| 200 | 成功 | 正常处理响应数据 |
| 400 | 请求错误 | 检查请求参数格式 |
| 404 | 资源不存在 | 检查请求URL是否正确 |
| 405 | 方法不允许 | 检查请求方法（GET/POST） |
| 500 | 服务器错误 | 稍后重试或联系技术支持 |

### 业务错误类型

| 错误类型 | 说明 | 触发场景 |
|---------|------|---------|
| Invalid request body | 请求体格式错误 | JSON格式不正确 |
| Failed to parse book info | 书籍解析失败 | Python脚本执行错误 |
| Failed to get book cover | 封面获取失败 | 图片源不可用 |
| book_name query parameter is required | 缺少必要参数 | 未提供book_name |

### 错误处理建议

```python
# Python 示例
import requests
import json

def parse_book_info(input_text):
    try:
        response = requests.post(
            "http://localhost:8080/v1/utils/parse-book-info",
            json={"input": input_text},
            timeout=30
        )
        response.raise_for_status()
        return response.json()
    except requests.exceptions.Timeout:
        print("请求超时")
    except requests.exceptions.HTTPError as e:
        print(f"HTTP错误: {e.response.status_code}")
        print(f"错误详情: {e.response.text}")
    except json.JSONDecodeError:
        print("响应JSON解析失败")
    except Exception as e:
        print(f"未知错误: {e}")
```

```javascript
// JavaScript 示例
async function parseBookInfo(input) {
  try {
    const response = await fetch('http://localhost:8080/v1/utils/parse-book-info', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ input }),
      signal: AbortSignal.timeout(30000) // 30秒超时
    });

    if (!response.ok) {
      const error = await response.text();
      throw new Error(`HTTP ${response.status_code}: ${error}`);
    }

    return await response.json();
  } catch (error) {
    if (error.name === 'AbortError') {
      console.error('请求超时');
    } else {
      console.error('错误:', error.message);
    }
  }
}
```

---

## 开发工具

### 在线测试

#### 1. cURL

**健康检查**
```bash
curl http://localhost:8080/health
```

**书籍解析**
```bash
curl -X POST http://localhost:8080/v1/utils/parse-book-info \
  -H "Content-Type: application/json" \
  -d '{"input": "三体"}'
```

**书籍封面**
```bash
curl "http://localhost:8080/v1/utils/book-cover?book_name=三体"
```

#### 2. 使用测试脚本

项目提供完整的测试脚本：

```bash
./test_api.sh
```

测试脚本会依次调用所有接口并输出结果。

#### 3. Postman

导入以下 Collection：

```json
{
  "info": {
    "name": "Oops Reader Backend API",
    "schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"
  },
  "item": [
    {
      "name": "健康检查",
      "request": {
        "method": "GET",
        "url": "http://localhost:8080/health"
      }
    },
    {
      "name": "书籍解析",
      "request": {
        "method": "POST",
        "url": "http://localhost:8080/v1/utils/parse-book-info",
        "header": [
          {
            "key": "Content-Type",
            "value": "application/json"
          }
        ],
        "body": {
          "mode": "raw",
          "raw": "{\"input\": \"三体\"}"
        }
      }
    },
    {
      "name": "书籍封面",
      "request": {
        "method": "GET",
        "url": {
          "raw": "http://localhost:8080/v1/utils/book-cover?book_name=三体",
          "query": [
            {
              "key": "book_name",
              "value": "三体"
            }
          ]
        }
      }
    }
  ]
}
```

#### 4. HTTPie

```bash
http GET http://localhost:8080/health

http POST http://localhost:8080/v1/utils/parse-book-info \
  input="三体" \
  --header "Content-Type: application/json"

http GET "http://localhost:8080/v1/utils/book-cover?book_name=三体"
```

### 本地开发

#### 启动服务

```bash
# 构建项目
go build -o bin/simple ./cmd/simple

# 启动服务
./bin/simple > /tmp/oops-server.log 2>&1 &

# 查看日志
tail -f /tmp/oops-server.log
```

#### 重启服务

```bash
# 停止服务
kill $(ps aux | grep 'bin/simple' | grep -v grep | awk '{print $2}')

# 重新构建
go build -o bin/simple ./cmd/simple

# 启动服务
./bin/simple > /tmp/oops-server.log 2>&1 &

# 验证启动
sleep 2
curl http://localhost:8080/health
```

#### 端口检查

```bash
# 检查端口是否被占用
lsof -i:8080

# 检查服务状态
ps aux | grep 'bin/simple'
```

### 调试技巧

#### 1. 查看响应头
```bash
curl -v http://localhost:8080/health
```

#### 2. 格式化输出
```bash
# 使用 jq 格式化 JSON
curl -s http://localhost:8080/health | jq '.'

# Python 格式化
curl -s http://localhost:8080/health | python3 -m json.tool
```

#### 3. 保存响应到文件
```bash
curl -s "http://localhost:8080/v1/utils/book-cover?book_name=三体" \
  > result.json
```

#### 4. 并发测试
```bash
# 使用 Apache Bench
ab -n 100 -c 10 http://localhost:8080/health

# 使用 wrk
wrk -t4 -c100 -d30s http://localhost:8080/health
```

### 监控和日志

#### 查看实时日志
```bash
tail -f /tmp/oops-server.log
```

#### 查看最近错误
```bash
grep -i error /tmp/oops-server.log | tail -20
```

#### 性能分析
```bash
# 使用 pprof（需要完整版本）
go tool pprof http://localhost:8080/debug/pprof/profile
```

---

## 最佳实践

### 1. 超时控制
- 书籍解析建议 30 秒超时
- 封面获取建议 20 秒超时
- 健康检查建议 5 秒超时

### 2. 重试机制
```python
import time
from requests.exceptions import RequestException

def fetch_with_retry(url, max_retries=3, delay=2):
    for attempt in range(max_retries):
        try:
            response = requests.get(url, timeout=20)
            response.raise_for_status()
            return response.json()
        except RequestException as e:
            if attempt == max_retries - 1:
                raise
            time.sleep(delay * (attempt + 1))
```

### 3. 结果缓存
由于书籍解析和封面获取可能耗时较长，建议在客户端实现缓存：

```python
import hashlib
import json
from pathlib import Path

class BookCache:
    def __init__(self, cache_dir='book_cache'):
        self.cache_dir = Path(cache_dir)
        self.cache_dir.mkdir(exist_ok=True)

    def get(self, key):
        cache_file = self.cache_dir / self._filename(key)
        if cache_file.exists():
            return json.load(open(cache_file))
        return None

    def set(self, key, data):
        cache_file = self.cache_dir / self._filename(key)
        json.dump(data, open(cache_file, 'w'), ensure_ascii=False)

    def _filename(self, key):
        return hashlib.md5(key.encode()).hexdigest() + '.json'

# 使用
cache = BookCache()
cached = cache.get('三体')
if cached:
    print('使用缓存:', cached)
else:
    # 调用API
    result = parse_book_info('三体')
    cache.set('三体', result)
```

### 4. 错误处理
- 统一错误处理逻辑
- 记录错误日志
- 提供用户友好的错误提示

### 5. 请求验证
在发送请求前验证输入：

```python
def validate_book_input(input_str):
    if not input_str or not input_str.strip():
        raise ValueError("输入不能为空")

    if len(input_str) > 200:
        raise ValueError("输入过长")

    # 清理输入
    return input_str.strip()
```

---

## 版本历史

### v1.0.0 (2025-03-16)
- ✅ 初始版本发布
- ✅ 健康检查接口
- ✅ 书籍解析接口
- ✅ 书籍封面接口
- ✅ 支持豆瓣、Google Books、百度图片

---

## 支持

### 文档
- [README.md](../README.md) - 项目概述
- [DEPLOYMENT.md](../DEPLOYMENT.md) - 部署文档
- [backend-architecture-and-db-design.md](./backend-architecture-and-db-design.md) - 架构设计

### 问题反馈
如有问题，请查看项目 Issues 或联系技术支持。

### 许可证
MIT License

---

**文档最后更新**: 2025-03-16
**文档版本**: v1.0.0
