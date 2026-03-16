#!/usr/bin/env python3
"""
热门书籍爬取与封面获取 - 一体化工具

使用流程：
1. 爬取近十年热门书籍榜单（可选）
2. 批量获取书籍封面

使用方法：
    # 完整流程
    python3 main.py

    # 仅爬取书籍榜单
    python3 main.py --step1-only

    # 仅获取封面（需要已有书单）
    python3 main.py --step2-only --book-list book_ranks/data/book_ranks.json
"""

import argparse
import sys
from pathlib import Path
from book_rank_fetcher import BookRankFetcher
from batch_cover_fetcher import BatchCoverFetcher

# 默认配置
DEFAULT_OUTPUT_DIR = "book_data"  # 主输出目录
DEFAULT_RANK_OUTPUT_DIR = "book_ranks"  # 书籍榜单输出目录
DEFAULT_COVER_OUTPUT_DIR = "book_covers"  # 封面输出目录
DEFAULT_BOOK_LIST = "book_ranks/book_ranks.json"  # 默认书单文件


def step1_fetch_book_ranks(output_dir: str):
    """
    步骤1：爬取书籍榜单
    """
    print("\n" + "="*70)
    print("步骤 1/2: 爬取近十年热门书籍榜单")
    print("="*70)

    fetcher = BookRankFetcher(output_dir=output_dir)
    fetcher.run()

    print("\n✅ 步骤1完成!")
    print(f"📁 书单文件: {output_dir}/book_ranks.json")
    print(f"📁 Web novels: {output_dir}/web_novels.csv")
    print(f"📁 Other books: {output_dir}/other_books.csv")


def step2_fetch_covers(book_list_file: str, output_dir: str, test_mode: bool = False):
    """
    步骤2：批量获取封面
    """
    print("\n" + "="*70)
    print("步骤 2/2: 批量获取书籍封面")
    print("="*70)

    book_list_path = Path(book_list_file)
    if not book_list_path.exists():
        print(f"\n❌ 错误: 书单文件不存在: {book_list_file}")
        print(f"\n💡 提示: 请先运行步骤1爬取书单，或者指定正确的书单文件")
        sys.exit(1)

    # 限制数量（测试模式）
    limit = 10 if test_mode else None
    if test_mode:
        print(f"\n[测试模式] 仅处理前 10 本书\n")

    fetcher = BatchCoverFetcher(
        book_list_file=book_list_file,
        output_dir=output_dir,
    )
    fetcher.run(limit=limit)

    print("\n✅ 步骤2完成!")
    print(f"📁 封面保存目录: {output_dir}")
    print(f"📁 失败列表: {output_dir}/failed_books.json")


def main():
    parser = argparse.ArgumentParser(
        description="热门书籍爬取与封面获取工具",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
示例用法:
  # 完整流程（先爬取书单，再获取封面）
  python3 main.py

  # 仅爬取书籍榜单
  python3 main.py --step1-only

  # 仅获取封面（指定书单）
  python3 main.py --step2-only --book-list custom_books.json

  # 测试模式（仅处理10本书）
  python3 main.py --test

  # 自定义输出目录
  python3 main.py -o /path/to/output

注意事项:
  1. 步骤1会爬取大量数据，可能需要较长时间
  2. 网络环境可能会影响爬取效果，建议使用稳定的网络
  3. 如果某些数据源不可用，脚本会跳过并继续处理其他数据源
  4. 建议先使用 --test 测试脚本是否正常工作
        """
    )

    parser.add_argument(
        '--step1-only',
        action='store_true',
        help="仅执行步骤1（爬取书籍榜单）"
    )

    parser.add_argument(
        '--step2-only',
        action='store_true',
        help="仅执行步骤2（获取封面）"
    )

    parser.add_argument(
        '--book-list',
        default=DEFAULT_BOOK_LIST,
        help=f"书单文件路径（默认: {DEFAULT_BOOK_LIST}）"
    )

    parser.add_argument(
        '-o', '--output',
        default=DEFAULT_OUTPUT_DIR,
        help=f"主输出目录（默认: {DEFAULT_OUTPUT_DIR}）"
    )

    parser.add_argument(
        '--test',
        action='store_true',
        help="测试模式（仅处理少量数据）"
    )

    parser.add_argument(
        '--rank-output',
        default=DEFAULT_RANK_OUTPUT_DIR,
        help=f"书籍榜单输出目录（默认: {DEFAULT_RANK_OUTPUT_DIR}）"
    )

    parser.add_argument(
        '--cover-output',
        default=DEFAULT_COVER_OUTPUT_DIR,
        help=f"封面输出目录（默认: {DEFAULT_COVER_OUTPUT_DIR}）"
    )

    args = parser.parse_args()

    # 确定执行哪些步骤
    run_step1 = args.step1_only or not args.step2_only
    run_step2 = args.step2_only or not args.step1_only

    try:
        # 步骤1：爬取书籍榜单
        if run_step1:
            rank_output = Path(args.output) / args.rank_output
            step1_fetch_book_ranks(str(rank_output))

        # 步骤2：获取封面
        if run_step2:
            # 如果只执行步骤2，使用用户指定的书单文件
            book_list = args.book_list if args.step2_only else str(Path(args.output) / args.rank_output / "book_ranks.json")
            cover_output = Path(args.output) / args.cover_output
            step2_fetch_covers(book_list, str(cover_output), test_mode=args.test)

        print("\n" + "="*70)
        print("🎉 全部完成!")
        print("="*70)

    except KeyboardInterrupt:
        print("\n\n⚠️ 用户中断")
        sys.exit(1)
    except Exception as e:
        print(f"\n\n❌ 错误: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)


if __name__ == "__main__":
    main()
