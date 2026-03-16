#!/usr/bin/env python3
"""
书籍信息解析服务
从豆瓣、Google Books等源搜索书籍信息
"""

import sys
import json
import requests
import argparse
from typing import Dict, Optional

class BookInfoParser:
    def __init__(self):
        self.headers = {
            'User-Agent': 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36'
        }

    def parse_input(self, input_str: str) -> Dict:
        """
        解析输入字符串，提取书名和作者

        Args:
            input_str: 输入字符串，可能包含书名和作者

        Returns:
            包含原始解析结果的字典
        """
        input_str = input_str.strip()

        result = {
            'input': input_str,
            'title': None,
            'author': None
        }

        if not input_str:
            return result

        # 尝试识别作者（简单策略：寻找常见模式）
        if '作者' in input_str or '作' in input_str:
            parts = input_str.split('/')
            if len(parts) > 1:
                result['title'] = parts[0].strip()
                result['author'] = parts[1].strip()
            else:
                result['title'] = input_str
        elif '/' in input_str:
            parts = input_str.split('/')
            result['title'] = parts[0].strip()
            if len(parts) > 1:
                result['author'] = parts[1].strip()
        else:
            result['title'] = input_str

        return result

    def search_douban(self, title: str, author: Optional[str] = None) -> Dict:
        """
        从豆瓣搜索书籍

        Args:
            title: 书名
            author: 作者名（可选）

        Returns:
            搜索结果
        """
        search_url = "https://book.douban.com/j/search_subjects"
        params = {
            'type': 'book',
            'tag': '',
            'sort': 'relevance',
            'page_limit': 5,
            'page_start': 0
        }

        query = title
        if author:
            query = f"{title} {author}"
        params['search_term_text'] = query

        try:
            response = requests.get(search_url, params=params, headers=self.headers, timeout=10)
            if response.status_code == 200:
                data = response.json()
                books = data.get('subjects', [])

                if books:
                    book = books[0]
                    return {
                        'source': 'douban',
                        'title': book.get('title'),
                        'author': book.get('author', [''])[0] if book.get('author') else None,
                        'isbn': book.get('isbn13'),
                        'url': book.get('url'),
                        'cover': book.get('cover'),
                        'rating': book.get('rate'),
                        'found': True
                    }
        except Exception:
            pass

        return {
            'source': 'douban',
            'title': title,
            'author': author,
            'found': False
        }

    def search_google_books(self, title: str, author: Optional[str] = None) -> Dict:
        """
        从Google Books搜索书籍

        Args:
            title: 书名
            author: 作者名（可选）

        Returns:
            搜索结果
        """
        search_url = "https://www.googleapis.com/books/v1/volumes"
        params = {
            'q': f'intitle:{title}',
            'maxResults': 1
        }

        if author:
            params['q'] = f'intitle:{title}+inauthor:{author}'

        try:
            response = requests.get(search_url, params=params, headers=self.headers, timeout=10)
            if response.status_code == 200:
                data = response.json()
                items = data.get('items', [])

                if items:
                    volume = items[0]
                    volume_info = volume.get('volumeInfo', {})

                    authors = volume_info.get('authors', [])
                    image_links = volume_info.get('imageLinks', {})

                    return {
                        'source': 'google_books',
                        'title': volume_info.get('title'),
                        'author': authors[0] if authors else None,
                        'isbn': None,
                        'url': volume_info.get('infoLink'),
                        'cover': image_links.get('thumbnail') or image_links.get('smallThumbnail'),
                        'rating': volume_info.get('averageRating'),
                        'found': True
                    }
        except Exception:
            pass

        return {
            'source': 'google_books',
            'title': title,
            'author': author,
            'found': False
        }

    def parse_book_info(self, input_str: str) -> Dict:
        """
        解析并搜索书籍信息的主函数

        Args:
            input_str: 输入字符串

        Returns:
            包含书籍信息的字典
        """
        parsed = self.parse_input(input_str)
        title = parsed.get('title', '')
        author = parsed.get('author')

        results = {
            'input': input_str,
            'parsed_title': title,
            'parsed_author': author,
            'search_results': []
        }

        if not title:
            results['error'] = 'No title found in input'
            return results

        # 优先搜索豆瓣
        douban_result = self.search_douban(title, author)
        results['search_results'].append(douban_result)

        if douban_result.get('found'):
            results['best_match'] = douban_result
        else:
            # 豆瓣未找到，尝试Google Books
            google_result = self.search_google_books(title, author)
            results['search_results'].append(google_result)

            if google_result.get('found'):
                results['best_match'] = google_result
            else:
                results['error'] = 'Book not found'

        return results


def main():
    parser = argparse.ArgumentParser(description='Parse book info from input string')
    parser.add_argument('input', help='Input string containing book name and optionally author')
    args = parser.parse_args()

    book_parser = BookInfoParser()
    result = book_parser.parse_book_info(args.input)

    print(json.dumps(result, ensure_ascii=False, indent=2))


if __name__ == '__main__':
    main()
