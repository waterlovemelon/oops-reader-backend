#!/usr/bin/env python3
"""
测试精确匹配和两轮查找逻辑
"""

import sys
from bookCover import BookCoverFetcher

def test_exact_match():
    """测试精确匹配"""
    fetcher = BookCoverFetcher(timeout=10, check_watermark=False, allow_watermark=True)

    print("=== 测试精确匹配逻辑 ===\n")

    print("[测试1] '雪中悍刀行' - 应该精确匹配")
    result = fetcher.fetch_from_qqread('雪中悍刀行')
    if result:
        print(f"  ✓ 书名: '{result['title']}' == '雪中悍刀行'")
    else:
        print(f"  ✗ 未找到")

    print("\n[测试2] '雪中' - 应该失败（不是完整书名）")
    result = fetcher.fetch_from_qqread('雪中')
    if result and result['title'] == '雪中':
        print(f"  ✗ 错误：应该返回None，但找到了 '{result['title']}'")
    else:
        print(f"  ✓ 正确：未找到精确匹配")

    print("\n=== 测试完成 ===")


def test_watermark_fallback():
    """测试水印降级逻辑"""
    print("\n\n=== 测试水印降级逻辑 ===\n")

    print("[测试3] 开启水印检查，查找 '雪中悍刀行'")
    print("预期：第一轮查找无水印封面（如果没有），第二轮允许有水印")
    fetcher = BookCoverFetcher(timeout=10, check_watermark=True, allow_watermark=False)
    result = fetcher.fetch_by_title('雪中悍刀行')
    if result:
        print(f"  ✓ 来源: {result['source']}")
        print(f"  ✓ 书名: {result['title']}")
        print(f"  ✓ 封面: {result.get('imageUrl', 'N/A')[:80]}...")
        if result.get('error'):
            print(f"  ✗ 错误: {result['error']}")
    else:
        print(f"  ✗ 未找到")

if __name__ == '__main__':
    test_exact_match()
    test_watermark_fallback()
