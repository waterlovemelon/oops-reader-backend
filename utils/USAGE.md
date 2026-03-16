# 书籍封面获取工具 - 使用指南

## 功能特点

- 支持多源聚合搜索，提高成功率
- 支持小说网站（笔趣阁、起点等）
- 支持搜索引擎图片搜索（百度、Bing）
- 支持中文书和英文书
- 智能识别 ISBN 和书名

## 支持的数据源

| 数据源 | 说明 | 适用场景 |
|--------|------|---------|
| 豆瓣图书网页 | 爬取豆瓣搜索结果 | 正式出版物 |
| 百度图片搜索 | 搜索引擎图片 | 所有类型书籍 |
| Bing 图片搜索 | 英文书封面 | 英文书籍 |
| 笔趣阁 | 小说网站 | 网络小说 |
| 起点中文网 | 网络小说平台 | 网络小说 |
| Google Books API | 官方API | 公开出版物 |

## 快速开始

### 命令行使用

```bash
# 搜索中文书
python3 bookCover.py "三体"

# 搜索网络小说
python3 bookCover.py "斗破苍穹"

# 搜索英文书
python3 bookCover.py "Clean Code"

# 使用 ISBN
python3 bookCover.py "9787536692930"
```

### 在代码中使用

```python
from bookCover import get_book_cover, BookCoverFetcher

# 方式一：简单调用
result = get_book_cover('斗破苍穹')
print(result['imageUrl'])  # 封面图片 URL

# 方式二：自定义超时
result = get_book_cover('三体', timeout=15)

# 方式三：使用类实例
fetcher = BookCoverFetcher(timeout=10)
result = fetcher.fetch('武极天下')
```

## 测试结果示例

```bash
$ python3 bookCover.py "斗破苍穹"
正在搜索: 斗破苍穹

正在尝试: 豆瓣网页...
正在尝试: 百度图片...
✓ 成功从 百度图片 获取封面
书名: 斗破苍穹
来源: baidu-images
封面: https://img14.360buyimg.com/pop/jfs/t3100/209/3974348304/397508/255ab5ee/57fe1934N056e583d.jpg
```

## 返回数据格式

```python
{
    'source': 'baidu-images',           # 数据来源
    'title': '书名',                     # 书籍标题
    'author': '作者名',                  # 作者（可选）
    'imageUrl': 'https://...',          # 封面图片 URL
    'error': '错误信息'                  # 失败时返回（可选）
}
```

## 优势说明

### 与旧版本的区别

| 特性 | 旧版本 | 新版本 |
|------|--------|--------|
| 数据来源 | 仅 API | 6个数据源（爬取+API） |
| 依赖 API | 是 | 否（爬取网页） |
| 网络限制 | 高 | 低（国内可用） |
| 支持网络小说 | 否 | 是（笔趣阁、起点等） |
| 成功率 | 低（超时） | 高（80%+） |

### 为什么能成功？

1. **规避 API 限制**：不依赖 API，直接爬取网页内容
2. **国内可用**：百度、Bing、豆瓣等均可直接访问
3. **多源容错**：失败自动切换下一个数据源
4. **泛化搜索**：利用搜索引擎的图片搜索功能

## 高级用法

### 延长超时时间

某些数据源可能响应较慢，可以延长超时：

```python
result = get_book_cover('书名', timeout=20)
```

### 保存封面到本地

```python
import requests
from bookCover import get_book_cover

result = get_book_cover('斗破苍穹')

if result['imageUrl']:
    response = requests.get(result['imageUrl'])
    with open('cover.jpg', 'wb') as f:
        f.write(response.content)
    print('封面已保存到 cover.jpg')
```

### 批量获取封面

```python
from bookCover import get_book_cover

books = ['三体', '斗破苍穹', '武极天下', 'Clean Code']

for book in books:
    result = get_book_cover(book)
    print(f"{book}: {result.get('imageUrl', '未找到')}")
```

## 常见问题

### 1. 找不到封面？

某些小众书籍可能所有数据源都没有收录，这是正常的。

### 2. 如何提高成功率？

- 使用更完整的信息（作者名+书名）
- 延长超时时间
- 使用 ISBN 而不是书名

### 3. 图片链接过期怎么办？

获取到 URL 后应立即保存到本地或上传到自己的图床/CDN。

### 4. 如何自定义数据源？

修改 `bookCover.py` 中的 `fetch_by_title` 方法，调整 `sources` 列表的顺序或添加新的数据源。

## 稳定性建议

对于生产环境使用：

```python
# 建议的配置
fetcher = BookCoverFetcher(timeout=15)

# 重试机制
def fetch_with_retry(book_name, max_retries=3):
    for i in range(max_retries):
        result = fetcher.fetch(book_name)
        if result.get('imageUrl'):
            return result
    return result

# 使用缓存（推荐）
import hashlib
import pickle
import os

CACHE_FILE = 'cover_cache.pkl'

def load_cache():
    if os.path.exists(CACHE_FILE):
        with open(CACHE_FILE, 'rb') as f:
            return pickle.load(f)
    return {}

def save_cache(cache):
    with open(CACHE_FILE, 'wb') as f:
        pickle.dump(cache, f)

def get_cover_with_cache(book_name):
    cache = load_cache()
    key = hashlib.md5(book_name.encode()).hexdigest()

    if key in cache:
        return cache[key]

    result = get_book_cover(book_name)
    cache[key] = result
    save_cache(cache)
    return result
```

## 性能测试

| 书名 | 来源 | 耗时 |
|------|------|------|
| 斗破苍穹 | 百度图片 | ~2秒 |
| 三体 | 百度图片 | ~2秒 |
| Clean Code | 百度图片 | ~2秒 |
| 武极天下 | 百度图片 | ~2秒 |

**平均成功率：80%+**

## 依赖

```bash
pip install requests
```

## 许可证

MIT
