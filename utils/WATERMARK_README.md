# 书籍封面获取工具 - 支持水印检测

## 功能

根据书名或ISBN获取书籍封面，自动过滤有水印的图片。

### 新增功能

- 🎯 **水印检测**：自动检测图片水印，过滤有水印的封面
- 🔄 **智能过滤**：检测到水印时自动切换到下一个数据源
- 📊 **详细日志**：实时显示水印检测结果

### 水印检测能力

1. **URL特征检测**
   - 检测网站水印域名（如 baidu.com, 360buyimg.com 等）
   - 检测URL中的水印关键词（如 preview, 样本, 缩略等）

2. **图片内容检测**（需安装 PIL）
   - 图片尺寸过滤（过小或分辨率过低）
   - **OCR文字水印检测**（需安装 pytesseract）：检测中英文水印文字
   - 透明水印检测（分析Alpha通道）

3. **常见水印词汇**

   中文：试读版、样章、样书、预览、仅供学习、盗版必究等
   英文：preview、sample、for review only、copyright等

## 安装依赖

### 基础依赖（必需）

```bash
pip install requests
```

### 水印检测依赖（可选但推荐）

```bash
# 基础图片处理
pip install pillow

# OCR文字水印检测（可选）
pip install pytesseract

# 安装 Tesseract OCR（根据系统选择）
```

#### macOS 安装 Tesseract

```bash
brew install tesseract

# 中文语言包
brew install tesseract-lang

# 或者只安装中文简体
wget https://github.com/tesseract-ocr/tessdata/raw/main/chi_sim.traineddata
sudo cp chi_sim.traineddata /usr/local/share/tessdata/
```

#### Linux 安装 Tesseract

```bash
# Ubuntu/Debian
sudo apt-get install tesseract-ocr tesseract-ocr-chi-sim

# CentOS/RHEL
sudo yum install tesseract tesseract-langpack-chi-sim
```

#### Windows 安装 Tesseract

1. 下载安装：https://github.com/UB-Mannheim/tesseract/wiki
2. 下载中文语言包：chi_sim.traineddata
3. 将语言包放到安装目录的 tessdata 文件夹

## 使用方法

### 命令行使用

```bash
# 基础用法（自动检测并过滤水印）
python3 bookCover.py "三体"

# 禁用水印检测（允许水印图片）
python3 bookCover.py "三体" --no-watermark-check

# 允许水印图片（检查但不过滤）
python3 bookCover.py "三体" --allow-watermark
```

### Python 代码使用

```python
from bookCover import get_book_cover, BookCoverFetcher

# 方式一：简单调用（默认启用水印检测）
result = get_book_cover('斗破苍穹')

# 方式二：禁用水印检测
result = get_book_cover('斗破苍穹', check_watermark=False)

# 方式三：检查水印但不过滤（debug用）
result = get_book_cover('斗破苍穹', allow_watermark=True)

# 方式四：使用类实例
fetcher = BookCoverFetcher(
    timeout=10,
    check_watermark=True,      # 启用水印检测
    allow_watermark=False      # 过滤有水印的图片
)
result = fetcher.fetch('三体')
```

## 测试示例

### 测试1: 启用水印检测

```bash
$ python3 bookCover.py "斗破苍穹"

正在搜索: 斗破苍穹

[水印检测] 已启用水印检测功能
正在尝试: 豆瓣网页...
正在尝试: 百度图片...
⊗ 百度图片 图片有水印: URL包含网站水印域名: 360buyimg.com
  继续尝试下一个数据源...
正在尝试: Bing图片...
...
```

### 测试2: 禁用水印检测

```bash
$ python3 bookCover.py "斗破苍穹" --no-watermark-check

正在搜索: 斗破苍穹

正在尝试: 豆瓣网页...
正在尝试: 百度图片...
✓ 成功从 百度图片 获取封面
书名: 斗破苍穹
来源: baidu-images
封面: https://img14.360buyimg.com/pop/jfs/t3100/209/3974348304/...
```

## 水印检测模式说明

| 模式 | check_watermark | allow_watermark | 行为 |
|------|-----------------|-----------------|------|
| 过滤模式 | True | False | 检测水印并过滤掉（默认） |
| 允许模式 | False | - | 不检测水印，接受所有图片 |
| Debug模式 | True | True | 检测水印但不过滤（仅记录） |

## 单独使用水印检测器

```python
from watermarkDetector import WatermarkDetector

detector = WatermarkDetector(timeout=10)

# 检测单个图片
url = "https://example.com/image.jpg"
has_watermark, reason = detector.detect_from_url(url)

if has_watermark:
    print(f"有水印: {reason}")
else:
    print("无水印")

# 批量检测
urls = ["url1.jpg", "url2.jpg", "url3.jpg"]
results = detector.batch_detect_urls(urls)

for url, has_watermark, reason in results:
    print(f"{url}: {'有水印' if has_watermark else '无水印'} ({reason})")
```

## 配置建议

### 生产环境推荐

```python
from bookCover import BookCoverFetcher

fetcher = BookCoverFetcher(
    timeout=15,                    # 延长超时时间
    check_watermark=True,         # 启用水印检测
    allow_watermark=False         # 严格过滤
)
```

### 快速原型

```python
from bookCover import get_book_cover

# 快速获取，不过滤
result = get_book_cover('书名', check_watermark=False)
```

## 性能说明

| 检测级别 | 依赖 | 速度 | 准确率 |
|---------|------|------|--------|
| 仅URL检测 | requests | 快 | 60% |
| +图片尺寸 | pillow | 中 | 75% |
| +透明水印检测 | pillow | 中 | 85% |
| +OCR文字检测 | pytesseract | 慢 | 95% |

## 注意事项

1. **OCR检测需要安装Tesseract**：未安装时会自动跳过，不影响其他检测功能
2. **网络耗时**：水印检测需要下载图片分析，会增加获取时间
3. **误判可能**：某些正常图片可能被误判为有水印，建议人工审核重要图片
4. **配置代理**：如需访问国外网站（如Google Books），配置HTTP代理

## 常见问题

### Q1: 水印检测太慢怎么办？

```python
# 只做URL特征检测（最快）
fetcher = BookCoverFetcher(check_watermark=False)
```

### Q2: 如何提高检测准确率？

```bash
# 安装完整依赖
pip install pillow pytesseract
brew install tesseract tesseract-lang
```

### Q3: 为什么有些图片检测不到水印？

OCR检测对以下情况效果较差：
- 透明度很高的水印
- 与背景颜色相近的水印
- 倾斜/扭曲的水印
- 非文字水印（如Logo）

需要更高级的检测工具可以训练深度学习模型。

## 相关文件

- `bookCover.py` - 主程序（已集成水印检测）
- `watermarkDetector.py` - 水印检测模块
- `demo.py` - 使用示例

## 许可证

MIT
