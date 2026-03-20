#!/usr/bin/env python3
import requests
import sys
import json
from urllib.parse import quote_plus


def get_book_cover(identifier: str, timeout: int = 10) -> dict:
    try:
        query = quote_plus(identifier)
        url = f"https://www.googleapis.com/books/v1/volumes?q={query}&maxResults=1"
        response = requests.get(url, headers={'User-Agent': 'Mozilla/5.0'}, timeout=timeout)
        
        if response.status_code == 200:
            data = response.json()
            if data.get('items'):
                book = data['items'][0]['volumeInfo']
                return {
                    'source': 'google-books',
                    'title': book.get('title', identifier),
                    'author': book.get('authors', [''])[0] if book.get('authors') else None,
                    'imageUrl': book.get('imageLinks', {}).get('thumbnail')
                }
    except:
        pass
    
    return {'source': 'none', 'title': identifier, 'error': '未找到封面'}


if __name__ == '__main__':
    result = get_book_cover(sys.argv[1] if len(sys.argv) > 1 else '')
    print(json.dumps(result, ensure_ascii=False, indent=2))
