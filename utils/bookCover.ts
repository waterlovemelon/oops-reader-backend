/**
 * Book Cover Fetcher
 * 根据书名或ISBN获取书籍封面
 */

interface BookCoverResult {
  source: 'douban' | 'google' | 'none';
  title: string;
  author?: string;
  imageUrl?: string;
  error?: string;
}

export class BookCoverFetcher {
  /**
   * 从豆瓣获取封面（中文书首选）
   */
  private async fetchFromDouban(searchTerm: string): Promise<BookCoverResult | null> {
    try {
      const query = encodeURIComponent(searchTerm);
      const response = await fetch(
        `https://api.douban.com/v2/book/search?q=${query}&count=1`
      );

      if (!response.ok) {
        return null;
      }

      const data = await response.json();

      if (data.books && data.books.length > 0) {
        const book = data.books[0];
        return {
          source: 'douban',
          title: book.title,
          author: book.author?.join(', '),
          imageUrl: book.images?.large || book.images?.medium || book.images?.small,
        };
      }

      return null;
    } catch (error) {
      console.error('豆瓣API错误:', error);
      return null;
    }
  }

  /**
   * 从 Google Books 获取封面
   */
  private async fetchFromGoogleBooks(searchTerm: string): Promise<BookCoverResult | null> {
    try {
      const query = encodeURIComponent(searchTerm);
      const response = await fetch(
        `https://www.googleapis.com/books/v1/volumes?q=${query}&fields=items(volumeInfo(title,authors,imageLinks/thumbnail))&maxResults=1`
      );

      if (!response.ok) {
        return null;
      }

      const data = await response.json();

      if (data.items && data.items.length > 0) {
        const book = data.items[0].volumeInfo;
        return {
          source: 'google',
          title: book.title,
          author: book.authors?.join(', '),
          imageUrl: book.imageLinks?.thumbnail,
        };
      }

      return null;
    } catch (error) {
      console.error('Google Books API错误:', error);
      return null;
    }
  }

  /**
   * 根据 ISBN 获取封面
   */
  async fetchByISBN(isbn: string): Promise<BookCoverResult> {
    if (!isbn) {
      throw new Error('ISBN 不能为空');
    }

    // 尝试 Google Books（ISBN 支持最好）
    const googleResult = await this.fetchFromGoogleBooks(`isbn:${isbn}`);
    if (googleResult) {
      return googleResult;
    }

    // 尝试豆瓣
    const doubanResult = await this.fetchFromDouban(isbn);
    if (doubanResult) {
      return doubanResult;
    }

    return {
      source: 'none',
      title: isbn,
      error: '未找到该 ISBN 对应的书籍',
    };
  }

  /**
   * 根据书名获取封面（中文优先）
   */
  async fetchByTitle(title: string): Promise<BookCoverResult> {
    if (!title || title.trim().length === 0) {
      throw new Error('书名不能为空');
    }

    // 优先尝试豆瓣（中文书库）
    const doubanResult = await this.fetchFromDouban(title);
    if (doubanResult) {
      return doubanResult;
    }

    // 备选 Google Books
    const googleResult = await this.fetchFromGoogleBooks(title);
    if (googleResult) {
      return googleResult;
    }

    return {
      source: 'none',
      title: title,
      error: '未找到该书籍的封面',
    };
  }

  /**
   * 根据 ISBN 或书名获取封面（智能判断）
   */
  async fetch(identifier: string): Promise<BookCoverResult> {
    // 判断是否为 ISBN（10位或13位数字）
    const isbnPattern = /^(?:\d{9}[\dXx]|\d{13})$/;
    if (isbnPattern.test(identifier.replace(/[-\s]/g, ''))) {
      return this.fetchByISBN(identifier);
    } else {
      return this.fetchByTitle(identifier);
    }
  }
}

/**
 * 简化函数：直接根据书名/ISBN获取封面
 */
export async function getBookCover(identifier: string): Promise<BookCoverResult> {
  const fetcher = new BookCoverFetcher();
  return fetcher.fetch(identifier);
}

// 如果直接运行此文件，进行测试
if (require.main === module) {
  async function test() {
    console.log('=== 书籍封面获取测试 ===\n');

    const fetcher = new BookCoverFetcher();

    // 测试用例
    const testCases = [
      { id: '三体', desc: '中文书名' },
      { id: '9787536692930', desc: 'ISBN (三体)' },
      { id: 'The Great Gatsby', desc: '英文书名' },
      { id: 'Clean Code', desc: '技术书' },
      { id: '不存在的书123456', desc: '不存在的书' },
    ];

    for (const testCase of testCases) {
      console.log(`测试: ${testCase.desc} (${testCase.id})`);
      try {
        const result = await fetcher.fetch(testCase.id);
        console.log(`  ✓ 来源: ${result.source}`);
        console.log(`  ✓ 书名: ${result.title}`);
        if (result.author) {
          console.log(`  ✓ 作者: ${result.author}`);
        }
        if (result.imageUrl) {
          console.log(`  ✓ 封面: ${result.imageUrl}`);
        }
        if (result.error) {
          console.log(`  ✗ 错误: ${result.error}`);
        }
      } catch (error) {
        console.log(`  ✗ 异常: ${error}`);
      }
      console.log();
    }
  }

  test().catch(console.error);
}
