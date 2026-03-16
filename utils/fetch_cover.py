#!/usr/bin/env python3
"""
获取书籍封面服务
"""

import sys
import json
import argparse
import subprocess

def get_book_cover(book_name: str) -> dict:
    try:
        result = subprocess.run(
            ["node", "bookCover.js", book_name],
            capture_output=True,
            text=True,
            timeout=15,
            cwd="/Users/jason/Workspace/Code/oops/reader/oops-reader-backend/utils"
        )

        if result.returncode == 0 and result.stdout:
            return json.loads(result.stdout)
        else:
            return {
                'source': 'none',
                'title': book_name,
                'error': 'Failed to fetch cover',
                'details': result.stderr
            }
    except subprocess.TimeoutExpired:
        return {
            'source': 'none',
            'title': book_name,
            'error': 'Timeout'
        }
    except Exception as e:
        return {
            'source': 'none',
            'title': book_name,
            'error': str(e)
        }


def main():
    parser = argparse.ArgumentParser(description='Get book cover')
    parser.add_argument('book_name', help='Book name or ISBN')
    args = parser.parse_args()

    result = get_book_cover(args.book_name)
    print(json.dumps(result, ensure_ascii=False, indent=2))


if __name__ == '__main__':
    main()
