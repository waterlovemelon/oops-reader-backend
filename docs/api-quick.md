# API 快速参考

**基础 URL**: `http://localhost:8080`

---

## 快速测试

```bash
# 健康检查
curl http://localhost:8080/health

# 书籍解析
curl -X POST http://localhost:8080/v1/utils/parse-book-info \
  -H "Content-Type: application/json" \
  -d '{"input": "三体"}'

# 书籍封面
curl "http://localhost:8080/v1/utils/book-cover?book_name=三体"
```

---

## 接口概览

| 接口 | 方法 | 路径 | 延时 | 说明 |
|-----|------|------|------|------|
| 健康检查 | GET | `/health` | <1s | 服务状态 |
| 书籍解析 | POST | `/v1/utils/parse-book-info` | 7-15s | 检索书籍信息 |
| 书籍封面 | GET | `/v1/utils/book-cover` | 5-10s | 获取封面URL |

---

## 详细说明

### 1. GET /health

检查服务健康状态

**响应**:
```json
{
  "message": "service is healthy",
  "status": "ok"
}
```

---

### 2. POST /v1/utils/parse-book-info

解析书籍名称并检索信息

**请求**:
```json
{
  "input": "三体"
}
```

**或带作者**:
```json
{
  "input": "1984/乔治·奥威尔"
}
```

**响应示例**:
```json
{
  "input": "三体",
  "parsed_title": "三体",
  "parsed_author": null,
  "search_results": [
    {
      "source": "google_books",
      "title": "The Three-Body Problem",
      "author": "Liu Cixin",
      "isbn": "9783533189460",
      "url": "...",
      "cover": "...",
      "rating": 4.5,
      "found": true
    }
  ],
  "best_match": { ... }
}
```

**数据源**: 豆瓣 → Google Books

---

### 3. GET /v1/utils/book-cover

获取书籍封面

**参数**: `book_name` (必填)

**响应示例**:
```json
{
  "source": "baidu-images",
  "title": "三体",
  "author": null,
  "imageUrl": "https://example.com/cover.jpg"
}
```

**来源**: 百度图片 → 豆瓣 → Google Books

---

## 错误码

| 状态码 | 说明 |
|--------|------|
| 200 | 成功 |
| 400 | 参数错误 |
| 404 | 接口不存在 |
| 500 | 服务器错误 |

---

## Python 示例

```python
import requests

# 健康检查
requests.get('http://localhost:8080/health').json()

# 书籍解析
response = requests.post(
    'http://localhost:8080/v1/utils/parse-book-info',
    json={'input': '三体'},
    timeout=30
).json()

# 书籍封面
requests.get(
    'http://localhost:8080/v1/utils/book-cover',
    params={'book_name': '三体'},
    timeout=20
).json()
```

---

## JavaScript 示例

```javascript
// 健康检查
await fetch('http://localhost:8080/health').then(r => r.json());

// 书籍解析
await fetch('http://localhost:8080/v1/utils/parse-book-info', {
  method: 'POST',
  headers: {'Content-Type': 'application/json'},
  body: JSON.stringify({input: '三体'}),
  signal: AbortSignal.timeout(30000)
}).then(r => r.json());

// 书籍封面
await fetch('http://localhost:8080/v1/utils/book-cover?book_name=三体', {
  signal: AbortSignal.timeout(20000)
}).then(r => r.json());
```

---

## 注意事项

⚠️ **超时控制**: 书籍解析建议30秒，封面获取建议20秒

⚠️ **网络依赖**: 依赖豆瓣、Google Books等外部服务

⚠️ **中文支持**: 完美支持中文书籍和作者

⚠️ **多数据源**: 自动切换数据源保证成功率

---

**完整文档**: [docs/api.md](./api.md)
