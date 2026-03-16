#!/bin/bash

# Oops Reader Backend 测试脚本

echo "========================================"
echo "Oops Reader Backend API 测试"
echo "========================================"
echo ""

BASE_URL="http://localhost:8080"

# 测试健康检查
echo "1. 测试健康检查接口..."
curl -s "${BASE_URL}/health" | jq '.'
echo ""
echo ""

# 测试书籍信息解析
echo "2. 测试书籍信息解析接口..."
curl -s -X POST "${BASE_URL}/v1/utils/parse-book-info" \
  -H "Content-Type: application/json" \
  -d '{"input": "三体"}' | jq '.'
echo ""
echo ""

# 测试书籍封面获取
echo "3. 测试书籍封面获取接口..."
curl -s "${BASE_URL}/v1/utils/book-cover?book_name=三体" | jq '.'
echo ""
echo ""

echo "========================================"
echo "测试完成"
echo "========================================"
