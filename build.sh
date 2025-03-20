#!/bin/bash

set -e  # 发生错误时退出

APP_NAME="cftun"
VERSION="2.1.2"
BUILD_TYPE="release"
BUILD_DIR="build"
PLATFORMS=("linux/amd64" "linux/arm64" "linux/arm" "windows/amd64" "windows/arm64" "darwin/amd64" "darwin/arm64")

# 创建 build 目录
mkdir -p $BUILD_DIR

# 交叉编译
for PLATFORM in "${PLATFORMS[@]}"; do
    OS=${PLATFORM%%/*}
    ARCH=${PLATFORM##*/}

    OUTPUT_NAME="$APP_NAME-$OS-$ARCH"
    if [ "$OS" == "windows" ]; then
        OUTPUT_NAME+=".exe"
    fi

    echo "Building for $OS/$ARCH..."
    LDFLAGS="-X main.Version=$VERSION -X main.BuildDate=$(date '+%Y-%m-%d_%H:%M:%S_%Z') -X main.BuildType=$BUILD_TYPE"

    env CGO_ENABLED=0 GOOS=$OS GOARCH=$ARCH go build -ldflags "$LDFLAGS" -o $BUILD_DIR/$OUTPUT_NAME

    # 压缩文件
    if [ "$OS" == "windows" ]; then
        zip -j "$BUILD_DIR/$APP_NAME-$OS-$ARCH.zip" "$BUILD_DIR/$OUTPUT_NAME"
    else
        tar -czvf "$BUILD_DIR/$APP_NAME-$OS-$ARCH.tar.gz" -C "$BUILD_DIR" "$OUTPUT_NAME"
    fi
done

echo "Build completed! Files are in the '$BUILD_DIR' directory."
