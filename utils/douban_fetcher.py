#!/usr/bin/env python3
"""
豆瓣图书Top250爬取器
从豆瓣图书Top250获取热门书籍信息

功能：
- 爬取豆瓣图书Top250
- 提取书名、作者、评分等信息
- 输出JSON和CSV格式
"""

import requests
import time
import random
import csv
import json
from pathlib import Path
from typing import List, Dict
from datetime import datetime
from urllib.parse import urljoin
import re

class DoubanBookFetcher:
    """豆瓣图书爬虫"""

    def __init__(self, output_dir: str = "book_ranks"):
        """
        初始化

        Args:
            output_dir: 输出目录
        """
        self.output_dir = Path(output_dir)
        self.output_dir.mkdir(parents=True, exist_ok=True)

        # 请求头
        self.headers = {
            'User-Agent': 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
            'Accept': 'text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8',
        }

        # 随机延迟（避免请求过快）
        self.min_delay = 2.0
        self.max_delay = 5.0

    def random_delay(self):
        """随机延迟"""
        delay = random.uniform(self.min_delay, self.max_delay)
        time.sleep(delay)

    def fetch_page(self, start: int = 0) -> List[Dict]:
        """
        获取一页书籍列表

        Args:
            start: 起始位置（0, 25, 50, ...）

        Returns:
            书籍列表
        """
        url = f"https://book.douban.com/top250?start={start}"
        print(f"[爬取] {url}")

        books = []

        try:
            response = requests.get(url, headers=self.headers, timeout=15)
            response.encoding = 'utf-8'

            if response.status_code != 200:
                print(f"[错误] HTTP {response.status_code}")
                return books

            # 提取书籍信息
            # 豆瓣Top250的HTML结构相对固定
            pattern = r'<tr class="item">.*?<td.*?valign="top">.*?<a.*?class.*?href="(.*?)".*?title="(.*?)">.*?<a.*?class="nbg".*?src="(.*?)"'

            # 更简单的模式匹配书名和链接
            # 每个书籍在一行中
            lines = response.text.split('\n')

            for line in lines:
                if 'class="pl2"' in line:
                    # 提取书名和链接
                    title_match = re.search(r'title="(.*?)"', line)
                    href_match = re.search(r'href="(.*?)"', line)

                    if title_match and href_match:
                        title = title_match.group(1)
                        href = href_match.group(1)

                        book_id = href.split('/')[-2] if '/subject/' in href else ''

                        books.append({
                            'book_id': book_id,
                            'book_name': title,
                            'url': href,
                            'source': '豆瓣',
                            'category': '豆瓣Top250',
                            'year': 2024,  # 豆瓣Top250是常青榜单
                            'rank': len(books) + start + 1,
                        })

                        if len(books) >= 100:  # 限制测试
                            break

            print(f"  [成功] 获取到 {len(books)} 本书")

        except Exception as e:
            print(f"[错误] {e}")

        return books

    def fetch_top250(self) -> List[Dict]:
        """
        获取豆瓣Top250全部书籍

        Returns:
            书籍列表
        """
        print("\n" + "="*60)
        print("豆瓣图书Top250爬取器")
        print("="*60)

        all_books = []

        # 豆瓣Top250有10页，每页25本
        for page in range(10):
            start = page * 25
            books = self.fetch_page(start)
            all_books.extend(books)

            if page < 9:  # 最后一页不延迟
                self.random_delay()

        print(f"\n[总计] 获取到 {len(all_books)} 本书")

        return all_books

    def save_to_json(self, data: List[Dict], filename: str = "douban_top250.json"):
        """
        保存到JSON文件

        Args:
            data: 书籍列表
            filename: 文件名
        """
        filepath = self.output_dir / filename

        output_data = {
            'generated_at': datetime.now().isoformat(),
            'source': '豆瓣图书Top250',
            'count': len(data),
            'books': data,
        }

        with open(filepath, 'w', encoding='utf-8') as f:
            json.dump(output_data, f, ensure_ascii=False, indent=2)

        print(f"\n[保存] JSON 文件已保存: {filepath}")

    def save_to_csv(self, data: List[Dict], filename: str = "douban_top250.csv"):
        """
        保存到CSV文件

        Args:
            data: 书籍列表
            filename: 文件名
        """
        filepath = self.output_dir / filename

        if not data:
            print(f"[警告] 没有数据可保存到CSV")
            return

        fieldnames = ['rank', 'book_name', 'book_id', 'category', 'year', 'source', 'url']

        with open(filepath, 'w', encoding='utf-8-sig', newline='') as f:
            writer = csv.DictWriter(f, fieldnames=fieldnames, extrasaction='ignore')
            writer.writeheader()
            writer.writerows(data)

        print(f"[保存] CSV 文件已保存: {filepath}")

    def run(self):
        """执行爬取任务"""
        # 爬取书籍
        books = self.fetch_top250()

        # 保存结果
        self.save_to_json(books)
        self.save_to_csv(books)

        # 统计信息
        print("\n" + "="*60)
        print("统计信息")
        print("="*60)
        print(f"总计: {len(books)} 本")
        print("="*60)


def main():
    """主函数"""
    fetcher = DoubanBookFetcher()
    fetcher.run()


if __name__ == "__main__":
    main()
