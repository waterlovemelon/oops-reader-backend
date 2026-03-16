#!/bin/bash

# Git Ignore 验证脚本
# 检查项目中是否有应该被忽略的文件被提交

echo "================================================"
echo "Git Ignore 验证工具"
echo "================================================"
echo ""

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 检查计数
ERRORS=0
WARNINGS=0

echo "1. 检查配置文件..."
echo "================================"

if [ -f "config.yaml" ] && git ls-files config.yaml | grep -q config.yaml; then
    echo -e "${RED}✗ ERROR${NC}: config.yaml 被提交到 Git（包含敏感信息）"
    echo "  解决: git rm --cached config.yaml"
    ((ERRORS++))
else
    echo -e "${GREEN}✓ OK${NC}: config.yaml 未被跟踪"
fi

if [ ! -f "config.yaml.example" ]; then
    echo -e "${YELLOW}⚠ WARNING${NC}: config.yaml.example 不存在"
    echo "  建议: 从 config.yaml 创建模板"
    ((WARNINGS++))
else
    echo -e "${GREEN}✓ OK${NC}: config.yaml.example 存在"
fi

echo ""
echo "2. 检查二进制文件..."
echo "================================"

if [ -d "bin" ] && [ "$(ls -A bin)" ]; then
    for file in bin/*; do
        if git ls-files "$file" | grep -q "$file"; then
            echo -e "${RED}✗ ERROR${NC}: $file 被提交到 Git（二进制文件）"
            echo "  解决: git rm --cached $file"
            ((ERRORS++))
        fi
    done
else
    echo -e "${GREEN}✓ OK${NC}: bin/ 目录为空或不存在"
fi

echo ""
echo "3. 检查环境变量文件..."
echo "================================"

ENV_FILES=(".env" ".env.local" ".env.production")
for env_file in "${ENV_FILES[@]}"; do
    if [ -f "$env_file" ] && git ls-files "$env_file" | grep -q "$env_file"; then
        echo -e "${RED}✗ ERROR${NC}: $env_file 被提交到 Git（敏感信息）"
        echo "  解决: git rm --cached $env_file"
        ((ERRORS++))
    fi
done

echo -e "${GREEN}✓ OK${NC}: 没有环境变量文件被跟踪"

echo ""
echo "4. 检查日志文件..."
echo "================================"

if find . -name "*.log" -o -name "logs" -type d | head -1 | grep -q .; then
    for log_file in $(find . -name "*.log" -o -name "*.log.*"); do
        if git ls-files "$log_file" | grep -q "$log_file"; then
            echo -e "${RED}✗ ERROR${NC}: $log_file 被提交到 Git（日志文件）"
            echo "  解决: git rm --cached $log_file"
            ((ERRORS++))
        fi
    done
else
    echo -e "${GREEN}✓ OK${NC}: 没有日志文件被跟踪"
fi

echo ""
echo "5. 检查 Python 缓存..."
echo "================================"

if [ -d "__pycache__" ] && find __pycache__ -type f | head -1 | grep -q .; then
    for pycache_file in $(find __pycache__ -type f); do
        if git ls-files "$pycache_file" | grep -q "$pycache_file"; then
            echo -e "${YELLOW}⚠ WARNING${NC}: $pycache_file 被跟踪（Python 缓存）"
            ((WARNINGS++))
        fi
    done
fi

if git ls-files "*.pyc" | head -1 | grep -q .; then
    echo -e "${YELLOW}⚠ WARNING${NC}: .pyc 文件被跟踪"
    ((WARNINGS++))
else
    echo -e "${GREEN}✓ OK${NC}: 没有Python缓存文件被跟踪"
fi

echo ""
echo "6. 检查密钥文件..."
echo "================================"

SECRET_PATTERNS=("*.pem" "*.key" "*.crt" "credentials.json" "secrets/")
PATTERN_FOUND=0

for pattern in "${SECRET_PATTERNS[@]}"; do
    if git ls-files "$pattern" | head -1 | grep -q .; then
        echo -e "${RED}✗ ERROR${NC}: 密钥文件被跟踪 ($pattern)"
        ((ERRORS++))
        PATTERN_FOUND=1
    fi
done

if [ $PATTERN_FOUND -eq 0 ]; then
    echo -e "${GREEN}✓ OK${NC}: 没有密钥文件被跟踪"
fi

echo ""
echo "7. 检查 .gitignore 文件..."
echo "================================"

if [ -f ".gitignore" ]; then
    echo -e "${GREEN}✓ OK${NC}: .gitignore 文件存在"

    echo ""
    echo "重要的忽略规则："
    grep -E "(config\.yaml|\.env|bin/|\*\.log)" .gitignore | sed 's/^/  /'
else
    echo -e "${RED}✗ ERROR${NC}: .gitignore 文件不存在"
    ((ERRORS++))
fi

echo ""
echo "8. 显示当前被跟踪的文件..."
echo "================================"

TRACKED_FILES=$(git ls-files 2>/dev/null | wc -l)
echo "当前被跟踪的文件数: $TRACKED_FILES"

echo ""
echo "如果需要，可以查看这些文件："
echo "  git ls-files"

echo ""
echo "9. 总结"
echo "================================"

if [ $ERRORS -eq 0 ] && [ $WARNINGS -eq 0 ]; then
    echo -e "${GREEN}✓ 所有检查通过！${NC}"
    echo "没有敏感文件或不应被跟踪的文件被提交。"
    exit 0
elif [ $ERRORS -eq 0 ]; then
    echo -e "${YELLOW}⚠ 发现 $WARNINGS 个警告${NC}"
    exit 0
else
    echo -e "${RED}✗ 发现 $ERRORS 个错误和 $WARNINGS 个警告${NC}"
    echo ""
    echo "请修复这些问题后再提交代码。"
    exit 1
fi
