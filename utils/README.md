# Book Cover Fetcher

根据书名或 ISBN 获取书籍封面的工具库。

## 功能特性

- 支持中文书和英文书的封面搜索
- 自动识别 ISBN（10位或13位）
- 多数据源支持：豆瓣（中文书首选）、Google Books
- 智能降级和结果聚合
- 可配置超时时间

## 安装

```bash
# 无需额外依赖，使用 Node.js 18+ 内置的 fetch API
node bookCover.js
```

## 快速开始

```javascript
const { getBookCover } = require('./bookCover');

// 方法一：直接调用（推荐）
async function demo() {
  // 根据书名获取
  const result1 = await getBookCover('三体');
  console.log(result1);

  // 根据 ISBN 获取
  const result2 = await getBookCover('9787536692930');
  console.log(result2);
}

demo();
```

```javascript
// 方法二：使用类实例
const { BookCoverFetcher } = require('./bookCover');

async function demo() {
  const fetcher = new BookCoverFetcher({ timeout: 8000 });

  // 根据书名获取（优选中文名称）
  const result1 = await fetcher.fetchByTitle('三体');
  console.log(result1);

  // 根据 ISBN 获取
  const result2 = await fetcher.fetchByISBN('9787536692930');
  console.log(result2);

  // 智能识别（自动判断是书名还是 ISBN）
  const result3 = await fetcher.fetch('三体');
  console.log(result3);
}

demo();
```

## 返回数据格式

```javascript
{
  source: 'douban' | 'google' | 'none',
  title: '书籍标题',
  author: '作者名（可选）',
  imageUrl: '封面图片URL（可选）',
  error: '错误信息（仅失败时）'
}
```

## 测试

```bash
node bookCover.js
```

## 网络限制说明

**⚠️ 重要提示**：本工具依赖外部 API，网络环境会影响使用体验：

1. **Google Books API**
   - 在中国大陆网络环境下可能无法访问或超时
   - 如需使用，请配置 VPN 或代理

2. **豆瓣 API**
   - 无需认证即可使用，但有访问频率限制
   - 可能返回 403 状态码，表示被限流

3. **解决方案**

   方案一：配置代理（推荐用于服务器部署）

   ```javascript
   # 设置环境变量（需要支持代理的 fetch 实现）
   export HTTP_PROXY=http://proxy.example.com:8080
   export HTTPS_PROXY=http://proxy.example.com:8080
   node bookCover.js
   ```

   方案二：使用 CDN 另存封面图片

   ```javascript
   // 获取到封面 URL 后，可以上传到自己的 CDN 或图床
   const { getBookCover } = require('./bookCover');
   const result = await getBookCover('三体');

   if (result.imageUrl) {
     // 将 result.imageUrl 保存到你的图床
     console.log('原始封面 URL:', result.imageUrl);
   }
   ```

   方案三：延长超时时间

   ```javascript
   const { BookCoverFetcher } = require('./bookCover');
   const fetcher = new BookCoverFetcher({ timeout: 15000 }); // 15秒超时
   ```

## API 文档

### `getBookCover(identifier)`

直接获取书籍封面的简化函数。

**参数：**
- `identifier` (String): 书名或 ISBN

**返回：** Promise<BookCoverResult>

### `BookCoverFetcher`

主类，提供更细粒度的控制。

**构造函数：**
```javascript
new BookCoverFetcher({ timeout: 5000 })  // timeout: 请求超时时间（毫秒）
```

**方法：**

- `fetch(identifier)`: 智能识别并获取封面
- `fetchByTitle(title)`: 根据书名获取（优先豆瓣）
- `fetchByISBN(isbn)`: 根据 ISBN 获取（优先 Google Books）

## 测试用例

当前测试包含以下书籍：

| 书名 | 类型 | 预期行为 |
|------|------|----------|
| 三体 | 中文书名 | 优先从豆瓣获取 |
| 9787536692930 | ISBN | 直接从 Google/豆瓣获取 |
| The Great Gatsby | 英文书名 | 优先从 Google Books 获取 |
| Clean Code | 技术书 | 从 Google Books 获取 |
| 不存在的书123456 | 不存在 | 返回错误信息 |

## 已知问题

1. 豆瓣 API 有访问频率限制，短时间内多次请求可能返回 403
2. Google Books API 在国内网络环境不稳定
3. 部分书籍没有封面图片，返回的 imageUrl 可能为空

## 许可证

MIT
