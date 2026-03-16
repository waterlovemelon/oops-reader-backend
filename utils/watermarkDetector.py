#!/usr/bin/env python3
"""
图片水印检测模块
检测图片中的各种类型水印：文字水印、Logo水印、透明水印等
"""

import requests
from typing import Optional, List, Tuple
from urllib.parse import urlparse
import sys


class WatermarkDetector:
    """水印检测器"""

    def __init__(self, timeout: int = 10):
        """
        初始化水印检测器

        Args:
            timeout: 图片下载超时时间（秒）
        """
        self.timeout = timeout

        # 常见中文水印关键词
        self.cn_watermark_keywords = [
            '试读版', '样章', '样书', '预览', '试看', '试读',
            '仅供学习', '学习交流', '仅供参考', '仅供参考',
            '盗版必究', '侵权必究',
            '版权所有', '版权声明',
            '封面来源于网络', '来源网络',
            '电子版', '电子书',
            '仅供个人学习', '学习使用',
            '严禁商用', '禁止商用',
            '扫描版', '翻拍', '高清扫描',
            '某站', '某app', '某小程序',
        ]

        # 常见英文水印关键词
        self.en_watermark_keywords = [
            'preview', 'advance copy', 'galley',
            'for review only', 'review copy',
            'uncorrected proof', 'advance reading',
            'sample', 'sample chapter',
            'copyright', 'all rights reserved',
            'not for sale', 'do not distribute',
            'preview edition', 'proof',
            'advance reader', 'arc',
            'ebook', 'electronic',
            'courtesy of', 'image from',
        ]

        # 常见水印域名
        self.watermark_domains = [
            'baidu.com', 'bing.com', 'xiaohongshu.com', '360buyimg.com',
            'bdstatic.com', 'weibo.com', 'jd.com', 'taobao.com',
        ]

    def _download_image(self, image_url: str) -> Optional[bytes]:
        """
        下载图片

        Args:
            image_url: 图片URL

        Returns:
            图片二进制数据
        """
        try:
            response = requests.get(image_url, timeout=self.timeout, stream=True)
            if response.status_code == 200:
                return response.content
            return None
        except Exception as e:
            print(f'[水印检测] 下载图片失败: {e}', file=sys.stderr)
            return None

    def detect_from_url(self, image_url: str) -> Tuple[bool, Optional[str]]:
        """
        从URL检测图片是否有水印

        Args:
            image_url: 图片URL

        Returns:
            (是否有水印, 原因)
        """
        # 步骤1: 检查URL中的水印特征
        has_watermark, reason = self._detect_from_url_pattern(image_url)
        if has_watermark:
            return True, reason

        # 步骤2: 尝试下载图片并检测
        image_data = self._download_image(image_url)
        if not image_data:
            return False, None

        # 步骤3: 检查文件大小（太小可能是缩略图）
        if len(image_data) < 5000:  # 小于5KB
            return True, '图片尺寸过小，可能是缩略图'

        # 步骤4: 检查图片内容（需要PIL库）
        try:
            from PIL import Image
            import io

            image = Image.open(io.BytesIO(image_data))

            # 检查4.1: 分辨率过低
            width, height = image.size
            if width < 200 or height < 300:
                return True, f'分辨率过低 ({width}x{height})'

            # 检查4.2: 使用OCR检测文字水印
            has_text_watermark = self._detect_text_watermark(image)
            if has_text_watermark:
                return True, '检测到文字水印'

            # 检查4.3: 检测透明水印（通过颜色分析）
            has_transparent_watermark = self._detect_transparent_watermark(image)
            if has_transparent_watermark:
                return True, '检测到透明水印'

        except ImportError:
            print('[水印检测] 未安装PIL库，跳过详细检测', file=sys.stderr)
        except Exception as e:
            print(f'[水印检测] 图片分析失败: {e}', file=sys.stderr)

        return False, None

    def _detect_from_url_pattern(self, image_url: str) -> Tuple[bool, Optional[str]]:
        """
        根据URL特征检测水印

        Args:
            image_url: 图片URL

        Returns:
            (是否有水印, 原因)
        """
        # 检查URL中是否包含水印域名
        parsed = urlparse(image_url)
        domain = parsed.netloc.lower()

        for watermark_domain in self.watermark_domains:
            if watermark_domain in domain:
                return True, f'URL包含网站水印域名: {watermark_domain}'

        # 检查URL中的水印关键词
        url_lower = image_url.lower()
        watermark_in_url = [
            'preview', 'sample', 'thumbnail', 'thumb', 'small', 'tiny',
            'watermark', 'logo', 'scan', 'camera', 'capture',
            '试读', '样板', '预览', '缩略',
        ]

        for keyword in watermark_in_url:
            if keyword in url_lower:
                return True, f'URL包含水印特征: {keyword}'

        return False, None

    def _detect_text_watermark(self, image) -> bool:
        """
        使用OCR检测文字水印

        Args:
            image: PIL Image对象

        Returns:
            是否检测到文字水印
        """
        try:
            import pytesseract

            # 尝试识别图片中的文字
            text = pytesseract.image_to_string(image, lang='chi_sim+eng')

            # 检查识别出的文本是否包含水印关键词
            for keyword in self.cn_watermark_keywords + self.en_watermark_keywords:
                if keyword in text:
                    print(f'[水印检测] 发现水印文字: {keyword}', file=sys.stderr)
                    return True

            return False

        except ImportError:
            # 没有安装pytesseract，跳过OCR检测
            return False
        except Exception as e:
            print(f'[水印检测] OCR检测失败: {e}', file=sys.stderr)
            return False

    def _detect_transparent_watermark(self, image) -> bool:
        """
        检测透明水印

        Args:
            image: PIL Image对象

        Returns:
            是否检测到透明水印
        """
        try:
            # 检查是否有透明通道
            if image.mode == 'RGBA':
                # 分析Alpha通道
                alpha = image.split()[-1]

                # 统计非完全不透明像素的比例
                pixels = list(alpha.getdata())
                semi_transparent = sum(1 for p in pixels if 0 < p < 255)
                ratio = semi_transparent / len(pixels)

                # 如果有大量半透明像素，可能有透明水印
                if ratio > 0.05:  # 超过5%的像素是半透明的
                    print(f'[水印检测] 检测到透明水印 (半透明比例: {ratio:.2%})', file=sys.stderr)
                    return True

            return False

        except Exception as e:
            print(f'[水印检测] 透明水印检测失败: {e}', file=sys.stderr)
            return False

    def batch_detect_urls(self, image_urls: List[str]) -> List[Tuple[str, bool, Optional[str]]]:
        """
        批量检测图片URL是否有水印

        Args:
            image_urls: 图片URL列表

        Returns:
            [(url, has_watermark, reason), ...]
        """
        results = []

        for url in image_urls:
            has_watermark, reason = self.detect_from_url(url)
            results.append((url, has_watermark, reason))

        return results


def filter_images_by_watermark(image_urls: List[str], allow_watermark: bool = False) -> List[str]:
    """
    过滤掉有水印的图片

    Args:
        image_urls: 图片URL列表
        allow_watermark: 是否允许水印图片（True则保留所有）

    Returns:
        无水印的图片URL列表
    """
    if allow_watermark:
        return image_urls

    detector = WatermarkDetector()
    filtered = []

    for url in image_urls:
        has_watermark, reason = detector.detect_from_url(url)
        if has_watermark:
            print(f'[水印过滤] 跳过有水印图片: {reason}', file=sys.stderr)
        else:
            filtered.append(url)

    return filtered


def main():
    """命令行测试"""
    if len(sys.argv) < 2:
        print('使用方法: python watermarkDetector.py <图片URL>')
        print('建议先安装依赖:')
        print('  pip install pillow requests pytesseract')
        print('  # macOS: brew install tesseract')
        print('  # Linux: sudo apt-get install tesseract-ocr tesseract-ocr-chi-sim')
        sys.exit(1)

    image_url = sys.argv[1]
    detector = WatermarkDetector()

    print(f'正在检测: {image_url}\n')

    has_watermark, reason = detector.detect_from_url(image_url)

    if has_watermark:
        print(f'✗ 有水印: {reason}')
    else:
        print('✓ 无水印')


if __name__ == '__main__':
    main()
