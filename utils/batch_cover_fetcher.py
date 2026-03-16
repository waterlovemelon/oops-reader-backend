#!/usr/bin/env python3
"""
书籍封面批量获取器

功能：
1. 从书单文件（JSON/CSV）读取书籍列表
2. 批量获取书籍封面（使用 bookCover.py）
3. 保存封面图片到本地
4. 生成失败重试列表
"""

import json
import csv
import time
import random
import traceback
from pathlib import Path
from typing import List, Dict, Optional, Tuple
from datetime import datetime
import requests
import urllib.parse

# 导入封面获取工具
try:
    from bookCover import BookCoverFetcher
    HAS_COVER_FETCHER = True
except ImportError:
    HAS_COVER_FETCHER = False
    print("[警告] 未找到 bookCover.py，封面获取功能不可用")


class BatchCoverFetcher:
    """批量封面获取器"""

    def __init__(
        self,
        book_list_file: str,
        output_dir: str = "book_covers",
        cover_base_url: str = None,
        timeout: int = 10,
        max_retries: int = 3,
        delay_range: Tuple[float, float] = (0.5, 2.0),
    ):
        """
        初始化

        Args:
            book_list_file: 书单文件路径（JSON 或 CSV）
            output_dir: 封面输出目录
            cover_base_url: 封面图片基础 URL（用于拼接相对路径）
            timeout: 请求超时时间（秒）
            max_retries: 最大重试次数
            delay_range: 随机延迟范围（秒），避免请求过快
        """
        self.book_list_file = Path(book_list_file)
        self.output_dir = Path(output_dir)
        self.output_dir.mkdir(exist_ok=True)

        self.cover_base_url = cover_base_url
        self.timeout = timeout
        self.max_retries = max_retries
        self.min_delay, self.max_delay = delay_range

        # 初始化封面获取器
        if HAS_COVER_FETCHER:
            self.cover_fetcher = BookCoverFetcher(
                timeout=timeout,
                allow_proxy=False,
                check_watermark=True,
                allow_watermark=False,  # 不允许有水印的图片
            )
        else:
            self.cover_fetcher = None

        # 统计信息
        self.stats = {
            'total': 0,
            'success': 0,
            'failed': 0,
            'skipped': 0,
        }

        # 失败列表
        self.failed_books = []

        # 请求头
        self.headers = {
            'User-Agent': 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
        }

    def random_delay(self):
        """随机延迟"""
        delay = random.uniform(self.min_delay, self.max_delay)
        time.sleep(delay)

    def load_book_list(self) -> List[Dict]:
        """
        加载书单

        Returns:
            书籍列表
        """
        if not self.book_list_file.exists():
            raise FileNotFoundError(f"书单文件不存在: {self.book_list_file}")

        books = []

        try:
            # 根据文件扩展名判断格式
            if self.book_list_file.suffix == '.json':
                books = self._load_json()
            elif self.book_list_file.suffix == '.csv':
                books = self._load_csv()
            else:
                raise ValueError(f"不支持的文件格式: {self.book_list_file.suffix}")

            print(f"[加载] 成功加载 {len(books)} 本书")

        except Exception as e:
            print(f"[错误] 加载书单失败: {e}")
            traceback.print_exc()

        return books

    def _load_json(self) -> List[Dict]:
        """加载 JSON 格式书单"""
        with open(self.book_list_file, 'r', encoding='utf-8') as f:
            data = json.load(f)

        # 处理不同的 JSON 结构
        if isinstance(data, dict) and 'data' in data:
            # 如果是 book_rank_fetcher.py 生成的格式
            result = []
            for key in ['web_novels', 'other_books']:
                result.extend(data['data'].get(key, []))
            return result
        elif isinstance(data, list):
            # 直接是书籍列表
            return data
        else:
            raise ValueError("未知的 JSON 结构")

    def _load_csv(self) -> List[Dict]:
        """加载 CSV 格式书单"""
        books = []

        with open(self.book_list_file, 'r', encoding='utf-8-sig') as f:
            reader = csv.DictReader(f)
            for row in reader:
                books.append(row)

        return books

    def download_image(self, url: str, save_path: Path) -> bool:
        """
        下载图片

        Args:
            url: 图片 URL
            save_path: 保存路径

        Returns:
            是否成功
        """
        for attempt in range(self.max_retries):
            try:
                response = requests.get(
                    url,
                    headers=self.headers,
                    timeout=self.timeout,
                    stream=True
                )

                if response.status_code == 200:
                    # 检查文件大小（太小的可能是错误页面）
                    content_length = int(response.headers.get('content-length', 0))
                    if content_length > 1000:  # 至少 1KB
                        with open(save_path, 'wb') as f:
                            for chunk in response.iter_content(chunk_size=8192):
                                f.write(chunk)
                        return True
                    else:
                        print(f"    [警告] 图片大小太小: {content_length} bytes")
                        return False
                else:
                    print(f"    [警告] HTTP {response.status_code}: {url}")

            except Exception as e:
                if attempt < self.max_retries - 1:
                    print(f"    [重试 {attempt + 1}/{self.max_retries}] {e}")
                    time.sleep(2 ** attempt)  # 指数退避
                else:
                    print(f"    [错误] 下载失败: {e}")
                    return False

        return False

    def fetch_cover_url(self, book_name: str, isbn: str = None) -> Optional[str]:
        """
        获取封面 URL

        Args:
            book_name: 书名
            isbn: ISBN（可选）

        Returns:
            封面 URL
        """
        if not HAS_COVER_FETCHER:
            print("[错误] bookCover.py 不可用")
            return None

        # 优先使用 ISBN
        identifier = isbn if isbn else book_name

        try:
            result = self.cover_fetcher.fetch(identifier)

            if result and result.get('imageUrl'):
                return result['imageUrl']
            else:
                print(f"    [未找到] {book_name}")
                return None

        except Exception as e:
            print(f"    [错误] 获取失败: {e}")
            return None

    def sanitize_filename(self, filename: str) -> str:
        """
        清理文件名，移除非法字符

        Args:
            filename: 原始文件名

        Returns:
            清理后的文件名
        """
        # 替换非法字符
        illegal_chars = '<>:"/\\|?*'
        for char in illegal_chars:
            filename = filename.replace(char, '_')

        # 限制长度
        if len(filename) > 200:
            filename = filename[:200]

        return filename.strip()

    def save_cover(self, book: Dict) -> Tuple[bool, Optional[str]]:
        """
        保存书籍封面

        Args:
            book: 书籍信息

        Returns:
            (是否成功, 错误信息)
        """
        book_name = book.get('book_name') or book.get('name') or book.get('title')
        if not book_name:
            return False, "缺少书名"

        print(f"[处理] {book_name}")

        # 获取封面 URL
        cover_url = book.get('cover_url') or book.get('imageUrl')

        if not cover_url:
            # 如果没有封面 URL，尝试使用 bookCover.py 获取
            isbn = book.get('isbn') or book.get('ISBN')
            cover_url = self.fetch_cover_url(book_name, isbn)

        if not cover_url:
            return False, "未找到封面 URL"

        # 生成保存路径
        # 格式: {category}/{book_name}.jpg
        category = book.get('category', 'other')
        category_dir = self.output_dir / category
        category_dir.mkdir(exist_ok=True)

        filename = self.sanitize_filename(book_name)
        # 尝试获取图片扩展名
        ext = '.jpg'  # 默认
        if cover_url:
            parsed = urllib.parse.urlparse(cover_url)
            path = parsed.path.lower()
            if path.endswith('.png'):
                ext = '.png'
            elif path.endswith('.webp'):
                ext = '.webp'

        save_path = category_dir / (filename + ext)

        # 下载封面
        success = self.download_image(cover_url, save_path)

        if success:
            print(f"  [成功] 已保存: {save_path.name}")
            return True, None
        else:
            return False, "下载失败"

    def run(self, limit: Optional[int] = None):
        """
        执行批量获取任务

        Args:
            limit: 限制处理数量（用于测试）
        """
        print("\n" + "="*60)
        print("书籍封面批量获取器")
        print("="*60)
        print(f"书单文件: {self.book_list_file}")
        print(f"输出目录: {self.output_dir}")
        print(f"超时时间: {self.timeout}秒")
        print(f"最大重试: {self.max_retries}次")
        print("="*60 + "\n")

        # 加载书单
        books = self.load_book_list()
        if not books:
            print("[错误] 没有书籍数据")
            return

        self.stats['total'] = len(books)

        # 限制数量（用于测试）
        if limit:
            books = books[:limit]
            print(f"[限制] 仅处理前 {limit} 本书\n")

        # 批量处理
        for i, book in enumerate(books, 1):
            print(f"\n[{i}/{len(books)}] ", end="")

            try:
                success, error = self.save_cover(book)

                if success:
                    self.stats['success'] += 1
                else:
                    self.stats['failed'] += 1
                    self.failed_books.append({
                        'book': book,
                        'error': error,
                    })

            except Exception as e:
                self.stats['failed'] += 1
                self.failed_books.append({
                    'book': book,
                    'error': str(e),
                })
                print(f"  [异常] {e}")

            # 随机延迟
            self.random_delay()

        # 保存失败列表
        if self.failed_books:
            failed_file = self.output_dir / "failed_books.json"
            with open(failed_file, 'w', encoding='utf-8') as f:
                json.dump({
                    'generated_at': datetime.now().isoformat(),
                    'count': len(self.failed_books),
                    'books': self.failed_books,
                }, f, ensure_ascii=False, indent=2)
            print(f"\n[保存] 失败列表已保存: {failed_file}")

        # 统计信息
        print("\n" + "="*60)
        print("执行完成")
        print("="*60)
        print(f"总计: {self.stats['total']} 本")
        print(f"成功: {self.stats['success']}")
        print(f"失败: {self.stats['failed']}")
        if self.stats['total'] > 0:
            success_rate = (self.stats['success'] / self.stats['total']) * 100
            print(f"成功率: {success_rate:.1f}%")
        print("="*60)


def main():
    """主函数"""
    import argparse

    parser = argparse.ArgumentParser(description="批量获取书籍封面")
    parser.add_argument('book_list', help="书单文件路径（JSON 或 CSV）")
    parser.add_argument('-o', '--output', default='book_covers', help="封面输出目录")
    parser.add_argument('-t', '--timeout', type=int, default=10, help="请求超时时间（秒）")
    parser.add_argument('-r', '--retries', type=int, default=3, help="最大重试次数")
    parser.add_argument('-l', '--limit', type=int, help="限制处理数量（用于测试）")
    parser.add_argument('--delay', type=float, nargs=2, default=[0.5, 2.0], help="随机延迟范围（秒）")

    args = parser.parse_args()

    fetcher = BatchCoverFetcher(
        book_list_file=args.book_list,
        output_dir=args.output,
        timeout=args.timeout,
        max_retries=args.retries,
        delay_range=tuple(args.delay),
    )
    fetcher.run(limit=args.limit)


if __name__ == "__main__":
    main()
