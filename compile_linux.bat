@echo off
rem CLIProxyAPI Windows 编译脚本 (用于编译 Linux 二进制文件)

echo 检查是否安装了 Go...
go version >nul 2>&1
if errorlevel 1 (
    echo 错误: Go 未安装，请先安装 Go 1.26 或更高版本
    exit /b 1
)

for /f "tokens=3" %%g in ('go version') do (
    set GO_VERSION=%%g
)
echo 检测到 Go 版本: %GO_VERSION%

rem 设置输出目录
set OUTPUT_DIR=.\bin
if not exist "%OUTPUT_DIR%" mkdir "%OUTPUT_DIR%"

rem 获取版本信息
set VERSION=dev
set COMMIT=unknown
set BUILD_DATE=%date:~0,4%-%date:~5,2%-%date:~8,2%T%time:~0,2%:%time:~3,2%:%time:~6,2%Z

rem 检查是否在 git 仓库中
git rev-parse --git-dir >nul 2>&1
if not errorlevel 1 (
    for /f "delims=" %%i in ('git describe --tags --always --dirty') do set VERSION=%%i
    for /f "delims=" %%i in ('git rev-parse --short HEAD') do set COMMIT=%%i
)

echo 版本信息:
echo   Version: %VERSION%
echo   Commit: %COMMIT%
echo   Build Date: %BUILD_DATE%

rem 设置环境变量以交叉编译为 Linux
set GOOS=linux
set GOARCH=amd64
set CGO_ENABLED=0

echo 开始编译 CLIProxyAPI 为 Linux...

rem 编译主程序
go build -ldflags="-s -w -X \"main.Version=%VERSION%\" -X \"main.Commit=%COMMIT%\" -X \"main.BuildDate=%BUILD_DATE%\" -X \"github.com/router-for-me/CLIProxyAPI/v6/internal/buildinfo.Version=%VERSION%\" -X \"github.com/router-for-me/CLIProxyAPI/v6/internal/buildinfo.Commit=%COMMIT%\" -X \"github.com/router-for-me/CLIProxyAPI/v6/internal/buildinfo.BuildDate=%BUILD_DATE%\"" -o "%OUTPUT_DIR%\CLIProxyAPI-linux-amd64" .\cmd\server\

if %ERRORLEVEL% EQU 0 (
    echo 编译成功!
    echo Linux 可执行文件已生成: %OUTPUT_DIR%\CLIProxyAPI-linux-amd64
    dir "%OUTPUT_DIR%"
) else (
    echo 编译失败!
    exit /b 1
)

rem 重置环境变量
set GOOS=
set GOARCH=
set CGO_ENABLED=