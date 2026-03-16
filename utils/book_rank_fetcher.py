#!/usr/bin/env python3
"""
热门书籍榜单爬取器 - 近十年畅销书榜单

功能：
1. 爬取2015-2024年热门书籍名单
   - 网络小说：每年 top 510（3个数据源 × 170本，已减少10倍）
   - 其他类书籍：每年 top 100（2个数据源 × 50本，已减少10倍）

支持的数据源：
- 起点中文网（网络小说）
- 晋江文学城（网络小说）
- 纵横中文网（网络小说）
- 豆瓣图书（综合类）
- 当当网（综合类）

输出：
- JSON 格式的书籍名单
- CSV 格式便于查看
"""

import requests
import json
import csv
import time
import random
from pathlib import Path
from typing import List, Dict, Optional
from datetime import datetime
from urllib.parse import urljoin

class BookRankFetcher:
    """书籍榜单爬取器"""

    def __init__(self, output_dir: str = "book_ranks"):
        """
        初始化

        Args:
            output_dir: 输出目录
        """
        self.output_dir = Path(output_dir).resolve()
        # 确保父目录存在
        self.output_dir.parent.mkdir(parents=True, exist_ok=True)
        self.output_dir.mkdir(exist_ok=True)

        # 请求头，模拟浏览器
        self.headers = {
            'User-Agent': 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
            'Accept': 'text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8',
            'Accept-Language': 'zh-CN,zh;q=0.9,en;q=0.8',
            'Accept-Encoding': 'gzip, deflate, br',
            'Connection': 'keep-alive',
        }

        # 随机延迟范围（秒），避免请求过快
        self.min_delay = 1.0
        self.max_delay = 3.0

        # 年份范围（近十年）
        self.years = list(range(2015, 2025))

    def random_delay(self):
        """随机延迟"""
        delay = random.uniform(self.min_delay, self.max_delay)
        time.sleep(delay)

    def fetch_qidian_rank(self, year: int, limit: int = 5000) -> List[Dict]:
        """
        获取起点中文网榜单

        Args:
            year: 年份
            limit: 获取数量

        Returns:
            书籍列表
        """
        print(f"[起点] 正在获取 {year} 年榜单...")

        books = []
        # 起点中文网有多个榜单（月票榜、推荐票榜、畅销榜等）
        # 这里使用畅销榜作为示例
        rank_types = ['sales', 'recommend', 'monthly']  # 畅销、推荐、月票

        for rank_type in rank_types:
            try:
                # API 接口（需要实际测试这个接口是否可用）
                url = "https://www.qidian.com/ajax/book/category"
                params = {
                    '_csrfToken': '',
                    'page': 1,
                    'pageSize': min(100, limit),
                    'gender': 'male',  # male/female
                    'catId': '-1',  # -1 表示全部分类
                }

                response = requests.get(url, headers=self.headers, params=params, timeout=10)
                data = response.json()

                if data.get('code') == 0:
                    for item in data.get('data', {}).get('list', []):
                        books.append({
                            'book_name': item.get('bookName'),
                            'author': item.get('authorName'),
                            'category': '网络小说',
                            'year': year,
                            'source': '起点中文网',
                            'rank_type': rank_type,
                            'rank': item.get('rank', 0),
                        })

                self.random_delay()

            except Exception as e:
                print(f"[起点] 错误: {e}")
                continue

        return books[:limit]

    def fetch_jjwxc_rank(self, year: int, limit: int = 5000) -> List[Dict]:
        """
        获取晋江文学城榜单

        Args:
            year: 年份
            limit: 获取数量

        Returns:
            书籍列表
        """
        print(f"[晋江] 正在获取 {year} 年榜单...")

        books = []

        # 晋江文学城的榜单通常在网页上，需要爬取
        # 这里提供一个框架，实际可能需要根据页面结构调整
        base_url = "https://www.jjwxc.net"

        try:
            # 晋江有排行榜页面
            url = f"{base_url}/onebook.php?novelid=0&channelid=1&category=-1"

            response = requests.get(url, headers=self.headers, timeout=10)
            response.encoding = 'utf-8'

            # 这里需要解析 HTML，提取书籍信息
            # 由于涉及到具体的网页结构，这里留空，需要后续完善

            self.random_delay()

        except Exception as e:
            print(f"[晋江] 错误: {e}")

        return books[:limit]

    def fetch_zongheng_rank(self, year: int, limit: int = 5000) -> List[Dict]:
        """
        获取纵横中文网榜单

        Args:
            year: 年份
            limit: 获取数量

        Returns:
            书籍列表
        """
        print(f"[纵横] 正在获取 {year} 年榜单...")

        books = []

        # 纵横中文网的榜单
        try:
            base_url = "https://www.zongheng.com"
            rank_url = f"{base_url}/rank.html"

            response = requests.get(rank_url, headers=self.headers, timeout=10)
            response.encoding = 'utf-8'

            # 解析网页，提取书籍信息
            # 需要根据实际网页结构调整

            self.random_delay()

        except Exception as e:
            print(f"[纵横] 错误: {e}")

        return books[:limit]

    def fetch_douban_rank(self, year: int, limit: int = 1000) -> List[Dict]:
        """
        获取豆瓣图书榜单

        Args:
            year: 年份
            limit: 获取数量

        Returns:
            书籍列表
        """
        print(f"[豆瓣] 正在获取 {year} 年榜单...")

        books = []

        try:
            # 豆瓣图书的榜单 API
            base_url = "https://movie.douban.com/chart"
            # 实际上豆瓣的图书榜单需要具体分析
            # 这里提供框架

            self.random_delay()

        except Exception as e:
            print(f"[豆瓣] 错误: {e}")

        return books[:limit]

    def fetch_dangdang_rank(self, year: int, limit: int = 1000) -> List[Dict]:
        """
        获取当当网图书榜单

        Args:
            year: 年份
            limit: 获取数量

        Returns:
            书籍列表
        """
        print(f"[当当] 正在获取 {year} 年榜单...")

        books = []

        try:
            base_url = "http://product.dangdang.com"
            # 当当网的畅销榜单
            rank_url = f"{base_url}/bestseller.html"

            response = requests.get(rank_url, headers=self.headers, timeout=10)
            response.encoding = 'gbk'  # 当当网使用 GBK 编码

            # 解析网页，提取书籍信息
            # 需要根据实际网页结构调整

            self.random_delay()

        except Exception as e:
            print(f"[当当] 错误: {e}")

        return books[:limit]

    def fetch_by_source(self, source: str, year: int, limit: int) -> List[Dict]:
        """
        根据数据源获取榜单

        Args:
            source: 数据源名称
            year: 年份
            limit: 获取数量

        Returns:
            书籍列表
        """
        fetchers = {
            'qidian': self.fetch_qidian_rank,
            'jjwxc': self.fetch_jjwxc_rank,
            'zongheng': self.fetch_zongheng_rank,
            'douban': self.fetch_douban_rank,
            'dangdang': self.fetch_dangdang_rank,
        }

        fetcher = fetchers.get(source)
        if fetcher:
            return fetcher(year, limit)
        return []

    def fetch_all_books(self) -> Dict[str, List[Dict]]:
        """
        获取所有年份的书籍榜单

        Returns:
            字典，包含所有数据源的书籍列表
        """
        all_books = {
            'web_novels': [],  # 网络小说
            'other_books': [],  # 其他类书籍
        }

        # 网络小说数据源
        novel_sources = ['qidian', 'jjwxc', 'zongheng']
        novel_limit_per_year = 170  # 每个来源 170 本，合计 510（减少10倍）

        # 其他书籍数据源
        other_sources = ['douban', 'dangdang']
        other_limit_per_year = 50  # 每个来源 50 本，合计 100（减少10倍）

        print(f"\n{'='*50}")
        print(f"开始爬取热门书籍榜单")
        print(f"年份范围: {self.years[0]}-{self.years[-1]}")
        print(f"{'='*50}\n")

        for year in self.years:
            print(f"\n>>> 处理 {year} 年 <<<")

            # 爬取网络小说
            for source in novel_sources:
                books = self.fetch_by_source(source, year, novel_limit_per_year)
                all_books['web_novels'].extend(books)
                print(f"  [{source}] 获取到 {len(books)} 本书")

            # 爬取其他类书籍
            for source in other_sources:
                books = self.fetch_by_source(source, year, other_limit_per_year)
                all_books['other_books'].extend(books)
                print(f"  [{source}] 获取到 {len(books)} 本书")

        return all_books

    def save_to_json(self, data: Dict, filename: str = "book_ranks.json"):
        """
        保存到 JSON 文件

        Args:
            data: 数据
            filename: 文件名
        """
        filepath = self.output_dir / filename

        output_data = {
            'generated_at': datetime.now().isoformat(),
            'years': self.years,
            'summary': {
                'web_novels_count': len(data['web_novels']),
                'other_books_count': len(data['other_books']),
                'total_count': len(data['web_novels']) + len(data['other_books']),
            },
            'data': data,
        }

        with open(filepath, 'w', encoding='utf-8') as f:
            json.dump(output_data, f, ensure_ascii=False, indent=2)

        print(f"\n[保存] JSON 文件已保存: {filepath}")

    def save_to_csv(self, data: List[Dict], filename: str, category: str = "书籍"):
        """
        保存到 CSV 文件

        Args:
            data: 数据列表
            filename: 文件名
            category: 类别
        """
        filepath = self.output_dir / filename

        if not data:
            print(f"[警告] 没有数据可保存到 CSV: {filename}")
            return

        # 确定 CSV 列
        fieldnames = ['book_name', 'author', 'category', 'year', 'source', 'rank']

        with open(filepath, 'w', encoding='utf-8-sig', newline='') as f:
            writer = csv.DictWriter(f, fieldnames=fieldnames, extrasaction='ignore')
            writer.writeheader()
            writer.writerows(data)

        print(f"[保存] CSV 文件已保存: {filepath}")

    def run(self):
        """执行爬取任务"""
        print("\n" + "="*60)
        print("热门书籍榜单爬取器")
        print("="*60)

        # 爬取所有书籍
        all_books = self.fetch_all_books()

        # 保存结果
        self.save_to_json(all_books)
        self.save_to_csv(all_books['web_novels'], "web_novels.csv", "网络小说")
        self.save_to_csv(all_books['other_books'], "other_books.csv", "其他书籍")

        # 统计信息
        print("\n" + "="*60)
        print("统计信息")
        print("="*60)
        print(f"网络小说: {len(all_books['web_novels'])} 本")
        print(f"其他书籍: {len(all_books['other_books'])} 本")
        print(f"总计: {len(all_books['web_novels']) + len(all_books['other_books'])} 本")
        print("="*60)


def main():
    """主函数"""
    fetcher = BookRankFetcher()
    fetcher.run()


if __name__ == "__main__":
    main()
