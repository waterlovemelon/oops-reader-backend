#!/usr/bin/env python3
"""
Book Cover Fetcher - 增强版
根据书名或ISBN获取书籍封面，支持多个网站爬取
支持水印检测，自动过滤有水印的图片

支持的网站：
- 豆瓣图书（通过网页爬取）
- 笔趣阁（小说网站）
- 百度图片搜索
- Bing 图片搜索
- 起点中文网
- 番茄小说（第三方API）
- QQ阅读（第三方API）
- Google Books
"""

import requests
import urllib.parse
import sys
import re
from typing import Optional, Dict, List, Tuple
from urllib.parse import quote_plus

try:
    from watermarkDetector import WatermarkDetector
    HAS_WATERMARK_DETECTOR = True
except ImportError:
    HAS_WATERMARK_DETECTOR = False


class BookCoverFetcher:
    def __init__(self, timeout: int = 10, allow_proxy: bool = False,
                 check_watermark: bool = True, allow_watermark: bool = False):
        """
        初始化

        Args:
            timeout: 请求超时时间（秒）
            allow_proxy: 是否使用代理（如果有配置环境变量）
            check_watermark: 是否检查水印
            allow_watermark: 是否允许水印图片（False则过滤掉有水印的图片）
        """
        self.timeout = timeout
        self.check_watermark = check_watermark
        self.allow_watermark = allow_watermark

        if HAS_WATERMARK_DETECTOR and check_watermark:
            self.watermark_detector = WatermarkDetector(timeout=timeout)
            print('[水印检测] 已启用水印检测功能', file=sys.stderr)
        else:
            self.watermark_detector = None
            if check_watermark and not HAS_WATERMARK_DETECTOR:
                print('[水印检测] 未安装水印检测模块，跳过水印检查', file=sys.stderr)

        self.headers = {
            'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36',
            'Accept': 'text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8',
            'Accept-Language': 'zh-CN,zh;q=0.9,en;q=0.8',
        }

    def _check_image_watermark(self, image_url: str) -> Tuple[bool, Optional[str]]:
        """
        检查图片是否有水印

        Args:
            image_url: 图片URL

        Returns:
            (是否有水印, 原因)
        """
        if not self.watermark_detector or self.allow_watermark:
            return False, None

        return self.watermark_detector.detect_from_url(image_url)

    def fetch_from_baidu_images(self, search_term: str) -> Optional[Dict]:
        """从百度图片搜索获取封面"""
        try:
            query = quote_plus(f"{search_term} 书封面")
            url = f"https://image.baidu.com/search/flip?tn=baiduimage&ie=utf-8&word={query}&ct=201326592&v=flip"

            response = requests.get(url, headers=self.headers, timeout=self.timeout)

            if '"objURL":"' in response.text:
                # 解析图片URL
                matches = re.findall(r'"objURL":"([^"]+)"', response.text)
                if matches:
                    image_url = matches[0]
                    # 解码URL
                    image_url = image_url.replace(r'\u003d', '=').replace(r'\u0026', '&')
                    return {
                        'source': 'baidu-images',
                        'title': search_term,
                        'imageUrl': image_url
                    }
            return None

        except Exception as e:
            print(f'[百度图片] 错误: {e}', file=sys.stderr)
            return None

    def fetch_from_bing_images(self, search_term: str) -> Optional[Dict]:
        """从 Bing 图片搜索获取封面"""
        try:
            query = quote_plus(f"{search_term} book cover")
            url = f"https://www.bing.com/images/search?q={query}&first=1&tsc=ImageBasicHover"

            response = requests.get(url, headers=self.headers, timeout=self.timeout)

            # 寻找图片链接
            matches = re.findall(r'src="([^"]+\.jpg)"', response.text)
            if matches:
                # 取第一个匹配的图片
                for img_url in matches:
                    if 'http' in img_url:
                        return {
                            'source': 'bing-images',
                            'title': search_term,
                            'imageUrl': img_url
                        }
            return None

        except Exception as e:
            print(f'[Bing图片] 错误: {e}', file=sys.stderr)
            return None

    def fetch_from_douban(self, search_term: str) -> Optional[Dict]:
        """从豆瓣网页获取封面（不使用API）"""
        try:
            query = quote_plus(search_term)
            url = f"https://search.douban.com/book/subject_search?search_text={query}"

            response = requests.get(url, headers=self.headers, timeout=self.timeout, allow_redirects=True)

            if response.status_code == 200:
                title_matches = re.findall(r'<a[^>]*class="title-text"[^>]*>([^<]+)</a>', response.text)
                img_matches = re.findall(r'<img[^>]*class="[^"]*\bsubject-cover\b[^"]*"[^>]*src="([^"]+)"', response.text)

                for i, title_match in enumerate(title_matches):
                    book_title = title_match.strip()
                    if book_title == search_term:
                        if len(img_matches) > i:
                            img_url = img_matches[i]
                            img_url = re.sub(r'/spic/s\d+/', '/lpic/', img_url)
                            return {
                                'source': 'douban-web',
                                'title': search_term,
                                'imageUrl': img_url
                            }
                        break
            return None

        except Exception as e:
            print(f'[豆瓣网页] 错误: {e}', file=sys.stderr)
            return None

    def fetch_from_biquge(self, search_term: str) -> Optional[Dict]:
        """从笔趣阁类网站获取封面"""
        biquge_sites = [
            'https://www.bqg4.com',
            'https://www.bqgsw.com',
            'https://www.xbiquwx.com',
            'https://www.xbqg.com',
        ]

        for site in biquge_sites:
            try:
                query = quote_plus(search_term)
                url = f"{site}/modules/article/search.php?searchtype=articlename&searchkey={query}"

                response = requests.get(url, headers=self.headers, timeout=self.timeout)

                if response.status_code == 200:
                    title_matches = re.findall(r'<a[^>]*href="[^"]*"[^>]*>([^<]+)</a>', response.text)
                    img_matches = re.findall(r'<img[^>]*src="([^"]+)"[^>]*alt="[^"]*', response.text)

                    for i, title_match in enumerate(title_matches):
                        book_title = title_match.strip()
                        if book_title == search_term:
                            for img_url in img_matches:
                                if 'cover' in img_url.lower() or img_url.endswith('.jpg') or img_url.endswith('.png'):
                                    if img_url.startswith('//'):
                                        img_url = 'https:' + img_url
                                    elif img_url.startswith('/'):
                                        img_url = site + img_url
                                    return {
                                        'source': f'biquge-{site.split("//")[1].split(".")[1]}',
                                        'title': search_term,
                                        'imageUrl': img_url
                                    }
                            break

            except Exception as e:
                continue

        return None

    def fetch_from_qidian(self, search_term: str) -> Optional[Dict]:
        """从起点中文网获取封面"""
        try:
            query = quote_plus(search_term)
            url = f"https://www.qidian.com/ajax/search?kw={query}"

            response = requests.get(url, headers=self.headers, timeout=self.timeout)

            if response.status_code == 200:
                try:
                    data = response.json()
                    if data and data.get('data', {}).get('books'):
                        books = data['data']['books']
                        for book in books:
                            book_name = book.get('name', '')
                            if book_name == search_term:
                                img_url = book.get('imgUrl')
                                if img_url:
                                    return {
                                        'source': 'qidian',
                                        'title': search_term,
                                        'imageUrl': img_url
                                    }
                except:
                    pass
            return None

        except Exception as e:
            print(f'[起点中文网] 错误: {e}', file=sys.stderr)
            return None

    def fetch_from_fanqie(self, search_term: str) -> Optional[Dict]:
        """从番茄小说获取封面（第三方API）"""
        try:
            query = quote_plus(search_term)
            url = f"https://novel.acgh.top/book/search?name={query}"

            response = requests.get(url, headers=self.headers, timeout=self.timeout)

            if response.status_code == 200:
                data = response.json()
                if data and data.get('data'):
                    books = data['data'] if isinstance(data['data'], list) else [data['data']]
                    if books and len(books) > 0:
                        for book in books:
                            book_name = book.get('Name') or book.get('book_name') or ''
                            if book_name == search_term:
                                img_url = book.get('ThumbUrl') or book.get('thumb_url')
                                if img_url:
                                    return {
                                        'source': 'fanqie',
                                        'title': search_term,
                                        'imageUrl': img_url
                                    }
            return None

        except Exception as e:
            print(f'[番茄小说] 错误: {e}', file=sys.stderr)
            return None

    def fetch_from_qqread(self, search_term: str) -> Optional[Dict]:
        """从QQ阅读获取封面（第三方API）"""
        try:
            query = quote_plus(search_term)
            url = f"http://140.143.165.56/searchNovel?title={query}"

            response = requests.get(url, headers=self.headers, timeout=self.timeout)

            if response.status_code == 200:
                data = response.json()
                if data and data.get('data') and data['data'].get('book_data'):
                    books = data['data']['book_data']
                    if books and len(books) > 0:
                        for book in books:
                            book_name = book.get('book_name', '')
                            if book_name == search_term:
                                img_url = book.get('thumb_url')
                                if img_url:
                                    return {
                                        'source': 'qqread',
                                        'title': search_term,
                                        'imageUrl': img_url
                                    }
            return None

        except Exception as e:
            print(f'[QQ阅读] 错误: {e}', file=sys.stderr)
            return None

    def fetch_from_google_books_api(self, search_term: str) -> Optional[Dict]:
        """从 Google Books API 获取封面"""
        try:
            query = quote_plus(search_term)
            url = (
                f"https://www.googleapis.com/books/v1/volumes?q={query}"
                f"&fields=items(volumeInfo(title,authors,imageLinks/thumbnail))&maxResults=10"
            )
            response = requests.get(url, headers=self.headers, timeout=self.timeout)

            if response.status_code == 200:
                data = response.json()
                if data.get('items'):
                    for item in data['items']:
                        book = item['volumeInfo']
                        book_title = book.get('title', '')
                        if book_title == search_term:
                            return {
                                'source': 'google-books',
                                'title': search_term,
                                'author': ', '.join(book.get('authors', [])) if book.get('authors') else None,
                                'imageUrl': book.get('imageLinks', {}).get('thumbnail')
                            }
            return None

        except Exception as e:
            print(f'[Google Books] 错误: {e}', file=sys.stderr)
            return None

    def fetch_by_title(self, title: str) -> Dict:
        """根据书名获取封面（多源聚合）"""
        if not title or not title.strip():
            raise ValueError('书名不能为空')

        sources = [
            ('豆瓣网页', self.fetch_from_douban),
            ('百度图片', self.fetch_from_baidu_images),
            ('Bing图片', self.fetch_from_bing_images),
            ('番茄小说', self.fetch_from_fanqie),
            ('QQ阅读', self.fetch_from_qqread),
            ('笔趣阁', self.fetch_from_biquge),
            ('起点中文网', self.fetch_from_qidian),
            ('Google Books', self.fetch_from_google_books_api),
        ]

        if self.check_watermark and not self.allow_watermark:
            print('[第一轮] 搜索无水印封面...', file=sys.stderr)
            for source_name, fetch_func in sources:
                print(f'正在尝试: {source_name}...', file=sys.stderr)
                result = fetch_func(title)
                if result and result.get('imageUrl'):
                    image_url = result['imageUrl']
                    has_watermark, reason = self._check_image_watermark(image_url)
                    if has_watermark:
                        print(f'⊗ {source_name} 图片有水印: {reason}', file=sys.stderr)
                        print(f'  继续尝试下一个数据源...', file=sys.stderr)
                        continue

                    print(f'✓ 成功从 {source_name} 获取无水印封面', file=sys.stderr)
                    return result

        if self.check_watermark:
            print('[第二轮] 搜索允许水印的封面...', file=sys.stderr)

        for source_name, fetch_func in sources:
            print(f'正在尝试: {source_name}...', file=sys.stderr)
            result = fetch_func(title)
            if result and result.get('imageUrl'):
                print(f'✓ 成功从 {source_name} 获取封面', file=sys.stderr)
                return result

        return {
            'source': 'none',
            'title': title,
            'error': f'已尝试 {len(sources)} 个数据源，均未找到该书籍的封面'
        }

    def fetch_by_isbn(self, isbn: str) -> Dict:
        """根据 ISBN 获取封面"""
        if not isbn:
            raise ValueError('ISBN 不能为空')

        # 优先 Google Books（ISBN支持最好）
        result = self.fetch_from_google_books_api(f'isbn:{isbn}')
        if result and result.get('imageUrl'):
            return result

        # 尝试其他方法
        return self.fetch_by_title(isbn)

    def fetch(self, identifier: str) -> Dict:
        """
        根据书名或 ISBN 获取封面（智能判断）

        Args:
            identifier: 书名或 ISBN

        Returns:
            包含封面信息的字典
        """
        isbn_pattern = r'^(?:\d{9}[\dXx]|\d{13})$'
        if re.match(isbn_pattern, identifier.replace('-', '').replace(' ', '')):
            return self.fetch_by_isbn(identifier)
        else:
            return self.fetch_by_title(identifier)


def get_book_cover(identifier: str, timeout: int = 10,
                    check_watermark: bool = True, allow_watermark: bool = False) -> Dict:
    """
    根据书名或 ISBN 获取封面（简化函数）

    Args:
        identifier: 书名或 ISBN
        timeout: 请求超时时间（秒）
        check_watermark: 是否检查水印
        allow_watermark: 是否允许水印图片（False则过滤掉有水印的图片）

    Returns:
        包含封面信息的字典
    """
    fetcher = BookCoverFetcher(timeout=timeout, check_watermark=check_watermark,
                               allow_watermark=allow_watermark)
    return fetcher.fetch(identifier)


def main():
    """命令行使用"""
    import argparse

    parser = argparse.ArgumentParser(description='书籍封面获取工具（支持水印检测）')
    parser.add_argument('identifier', help='书名或ISBN')
    parser.add_argument('--no-watermark-check', action='store_true',
                        help='跳过水印检测')
    parser.add_argument('--allow-watermark', action='store_true',
                        help='允许水印图片（检查但不过滤）')

    args = parser.parse_args()

    identifier = args.identifier
    print(f'正在搜索: {identifier}\n', file=sys.stderr)

    result = get_book_cover(identifier,
                            check_watermark=not args.no_watermark_check,
                            allow_watermark=args.allow_watermark)

    print(f'书名: {result.get("title")}')
    print(f'来源: {result.get("source")}')

    if result.get('author'):
        print(f'作者: {result.get("author")}')

    if result.get('imageUrl'):
        print(f'封面: {result.get("imageUrl")}')

    if result.get('error'):
        print(f'错误: {result.get("error")}')


if __name__ == '__main__':
    main()
