# 热门书籍爬取与封面获取工具

一套完整的工具，用于爬取近十年热门书籍榜单并批量获取书籍封面。

## 功能特性

1. **书籍榜单爬取**
   - 爬取2015-2024年热门书籍名单
   -网络小说：每年 top 510（累计约5100本，已减少10倍）
   - 其他类书籍：每年 top 100（累计约1000本，已减少10倍）
   - 支持多个数据源：起点中文网、晋江文学城、纵横中文网、豆瓣、当当网
   - 输出 JSON 和 CSV 格式

2. **封面批量获取**
   - 从书单文件读取书籍列表
   - 批量获取书籍封面
   - 支持自动降级和错误重试
   - 智能水印检测（过滤有水印的图片）
   - 分类保存封面图片
   - 生成失败重试列表

## 安装依赖

```bash
# 系统依赖
pip install requests

# 运行主脚本可能需要的其他依赖已经内置
```

## 快速开始

### 1. 完整流程（推荐）

```bash
# 一键执行：先爬取书单，再获取封面
python main.py

# 使用自定义输出目录
python main.py -o /path/to/output

# 测试模式（仅处理10本书，快速验证）
python main.py --test
```

### 2. 分步执行

#### 步骤1：仅爬取书籍榜单

```bash
python main.py --step1-only
```

此步骤会生成：
- `book_rank/book_ranks.json` - 完整的书籍数据（JSON格式）
- `book_rank/web_novels.csv` - 网络小说（CSV格式）
- `book_rank/other_books.csv` - 其他书籍（CSV格式）

#### 步骤2：仅获取封面

```bash
# 使用默认书单（需先运行步骤1）
python main.py --step2-only

# 使用自定义书单
python main.py --step2-only --book-list custom_books.json

# 限制处理数量（测试用）
python main.py --step2-only --book-list book_ranks.json --limit 100
```

此步骤会生成：
- `book_covers/网络小说/[书名].jpg` - 网络小说封面
- `book_covers/other/[书名].jpg` - 其他书籍封面
- `book_covers/failed_books.json` - 失败列表（可用于重试）

## 高级用法

### 单独使用书籍榜单爬取器

```bash
python book_rank_fetcher.py
```

### 单独使用封面批量获取器

```bash
# 基本用法
python batch_cover_fetcher.py book_ranks.json

# 指定输出目录
python batch_cover_fetcher.py book_ranks.json -o my_covers

# 自定义超时和重试
python batch_cover_fetcher.py book_ranks.json -t 15 -r 5

# 限制处理数量
python batch_cover_fetcher.py book_ranks.json --limit 50
```

## 数据格式

### 书单文件格式（JSON）

```json
{
  "generated_at": "2024-01-01T00:00:00Z",
  "years": [2015, 2016, ..., 2024],
  "summary": {
    "web_novels_count": 5100,
    "other_books_count": 1000,
    "total_count": 6100
  },
  "data": {
    "web_novels": [
      {
        "book_name": "三体",
        "author": "刘慈欣",
        "category": "网络小说",
        "year": 2015,
        "source": "起点中文网",
        "rank_type": "sales",
        "rank": 1
      }
    ],
    "other_books": [...]
  }
}
```

### 书单文件格式（CSV）

```csv
book_name,author,category,year,source,rank
三体,刘慈欣,网络小说,2015,起点中文网,1
```

## 配置选项

### 书籍榜单爬取器配置

在 `book_rank_fetcher.py` 中可以修改：

```python
fetcher = BookRankFetcher(output_dir="book_ranks")

# 修改延迟时间（避免请求过快）
fetcher.min_delay = 1.0  # 秒
fetcher.max_delay = 3.0  # 秒

# 修改年份范围
fetcher.years = [2020, 2021, 2022]  # 只爬取3年
```

### 封面批量获取器配置

```bash
# 命令行参数
python batch_cover_fetcher.py \
    book_ranks.json \
    -o output_dir \              # 输出目录
    -t 10 \                      # 超时时间（秒）
    -r 3 \                       # 最大重试次数
    --delay 0.5 2.0              # 随机延迟范围（秒）
```

## 常见问题

### 1. 爬取速度慢怎么办？

- 修改 `random_delay` 范围，减少延迟时间
- 使用更稳定的网络环境
- 某些数据源可能较慢，脚本会自动跳过不可用的数据源

### 2. 封面获取失败率高？

- 检查网络连接
- 增加超时时间（`-t` 参数）
- 增加重试次数（`-r` 参数）
- 查看 `failed_books.json` 了解失败的书籍和原因

### 3. 某些数据源无法访问？

- 有些网站可能有反爬虫机制
- 脚本会自动跳过错误的数据源
- 不影响其他数据源的爬取

### 4. 如何只爬取特定年份？

修改 `book_rank_fetcher.py` 中的 `self.years`：

```python
# 只爬取2020-2022
self.years = [2020, 2021, 2022]
```

### 5. 如何处理失败的书籍？

失败列表保存在 `book_covers/failed_books.json`，可以：

1. 手动修复后重新运行
2. 将失败的书籍列表提取出来，单独处理
3. 检查失败原因，针对性调整参数

## 输出说明

### 目录结构

```
book_data/
├── book_ranks/
│   ├── book_ranks.json          # 完整数据（JSON）
│   ├── web_novels.csv           # 网络小说（CSV）
│   └── other_books.csv          # 其他书籍（CSV）
└── book_covers/
    ├── 网络小说/
    │   ├── 三体.jpg
    │   └── ...
    ├── other/
    │   ├── 设计模式.jpg
    │   └── ...
    └── failed_books.json        # 失败列表
```

## 注意事项

1. **网络依赖**
   - 脚本依赖外部网站和数据API
   - 网络环境会影响爬取效果
   - 建议使用稳定的网络

2. **反爬虫和限流**
   - 某些网站有访问频率限制
   - 脚本已内置随机延迟
   - 避免短时间内多次运行

3. **数据准确性**
   - 不同数据源的榜单可能有差异
   - 年份是基于爬取时间估算的
   - 建议交叉验证数据

4. **存储空间**
   - 6100本书的封面可能需要数GB空间（已减少10倍）
   - 建议提前规划存储位置
   - 可以先测试少量数据再全量爬取

## 许可证

MIT

## 贡献

欢迎提交 Issue 和 Pull Request！
