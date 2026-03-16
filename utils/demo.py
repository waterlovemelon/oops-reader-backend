#!/usr/bin/env python3
"""
使用示例
"""

from bookCover import get_book_cover, BookCoverFetcher
from watermarkDetector import WatermarkDetector
import requests


def example_1():
    """示例 1：最简单的用法"""
    result = get_book_cover('斗破苍穹')
    print(f"书名: {result['title']}")
    print(f"来源: {result['source']}")
    print(f"封面: {result.get('imageUrl', '未找到')}")


def example_2():
    """示例 2：使用 ISBN"""
    result = get_book_cover('9787536692930')
    print(f"书名: {result['title']}")
    print(f"作者: {result.get('author', '未知')}")
    print(f"封面: {result.get('imageUrl', '未找到')}")


def example_3():
    """示例 3：自定义超时时间"""
    fetcher = BookCoverFetcher(timeout=15)
    result = fetcher.fetch('三体')
    if result.get('imageUrl'):
        print(f"找到封面: {result['imageUrl']}")
    else:
        print(f"未找到封面: {result.get('error')}")


def example_4():
    """示例 4：保存封面到本地"""
    result = get_book_cover('武极天下')

    if result.get('imageUrl'):
        print(f"正在下载封面: {result['imageUrl']}")

        try:
            response = requests.get(result['imageUrl'], timeout=10)
            if response.status_code == 200:
                filename = f"{result['title']}_cover.jpg"
                with open(filename, 'wb') as f:
                    f.write(response.content)
                print(f"✓ 封面已保存到: {filename}")
            else:
                print(f"✗ 下载失败: HTTP {response.status_code}")
        except Exception as e:
            print(f"✗ 下载失败: {e}")
    else:
        print(f"未找到封面: {result.get('error')}")


def example_5():
    """示例 5：批量获取封面"""
    books = ['三体', '斗破苍穹', '武极天下', '完美世界']

    print("=== 批量获取封面 ===")
    for book in books:
        result = get_book_cover(book)
        print(f"\n{book}:")
        print(f"  来源: {result['source']}")
        if result.get('imageUrl'):
            print(f"  ✓ {result['imageUrl'][:80]}...")
        else:
            print(f"  ✗ {result.get('error')}")


def example_6():
    """示例 6：水印检测"""
    result = get_book_cover('斗破苍穹')

    if result.get('imageUrl'):
        print(f"检测: {result['imageUrl'][:80]}...")

        detector = WatermarkDetector()
        has_watermark, reason = detector.detect_from_url(result['imageUrl'])

        if has_watermark:
            print(f"✗ 有水印: {reason}")
        else:
            print("✓ 无水印")


def example_7():
    """示例 7：对比有/无水印检测"""
    book_name = '斗破苍穹'

    print(f"=== 书名: {book_name} ===\n")

    # 无水印检测
    print("1) 无水印检测:")
    result = get_book_cover(book_name, check_watermark=False)
    if result.get('imageUrl'):
        print(f"   获取成功: {result['source']}")
        print(f"   URL: {result['imageUrl'][:80]}...")

    # 有水印检测
    print("\n2) 有水印检测:")
    result = get_book_cover(book_name, check_watermark=True)
    if result.get('imageUrl'):
        print(f"   获取成功: {result['source']}")
        print(f"   URL: {result['imageUrl'][:80]}...")
    else:
        print(f"   获取失败: {result.get('error')}")


if __name__ == '__main__':
    import sys

    print("=" * 50)
    print("书籍封面获取工具 - 使用示例")
    print("=" * 50)

    if len(sys.argv) > 1:
        # 命令行直接输入书名
        book_name = ' '.join(sys.argv[1:])
        print(f"\n正在搜索: {book_name}\n")

        result = get_book_cover(book_name)

        print(f"书名: {result['title']}")
        print(f"来源: {result['source']}")

        if result.get('author'):
            print(f"作者: {result['author']}")

        if result.get('imageUrl'):
            print(f"封面: {result['imageUrl']}")

        if result.get('error'):
            print(f"错误: {result['error']}")
    else:
        print("\n请选择示例:")
        print("  1. 简单获取封面")
        print("  2. 使用 ISBN")
        print("  3. 自定义超时")
        print("  4. 保存封面到本地")
        print("  5. 批量获取封面")
        print("  6. 水印检测")
        print("  7. 对比有/无水印检测")
        print("\n或者直接运行:")
        print("  python demo.py 书名")
        print("  python demo.py 斗破苍穹")
        print("  python demo.py 三体")

        # 运行示例1
        print("\n" + "=" * 50)
        print("运行示例 1...")
        print("=" * 50)
        example_1()
