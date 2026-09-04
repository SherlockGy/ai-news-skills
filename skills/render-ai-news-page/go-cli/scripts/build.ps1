[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$OutputEncoding = [System.Text.UTF8Encoding]::new()
[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new()
$logPrefix = '[buildRenderAiNewsPage AI 新闻单页面构建]'
$cliRoot = Split-Path -Parent $PSScriptRoot
$binDirectory = Join-Path $cliRoot 'bin'
$hadGoos = Test-Path Env:GOOS
$previousGoos = $env:GOOS
$hadGoarch = Test-Path Env:GOARCH
$previousGoarch = $env:GOARCH
$hostGoos = (go env GOHOSTOS).Trim()
$hostGoarch = (go env GOHOSTARCH).Trim()

Push-Location $cliRoot
try {
	# 测试必须在当前主机运行，不继承开发者为交叉编译预设的目标平台。
	$env:GOOS = $hostGoos
	$env:GOARCH = $hostGoarch
    Write-Host "$logPrefix 运行测试"
    go test ./...

    New-Item -ItemType Directory -Force -Path $binDirectory | Out-Null
    $targets = @(
        @{ GOOS = 'windows'; Output = 'render-ai-news-page-windows-amd64.exe' },
        @{ GOOS = 'linux'; Output = 'render-ai-news-page-linux-amd64' },
        @{ GOOS = 'darwin'; Output = 'render-ai-news-page-darwin-amd64' }
    )

    foreach ($target in $targets) {
        $env:GOOS = $target.GOOS
        $env:GOARCH = 'amd64'
        $outputPath = Join-Path $binDirectory $target.Output
        Write-Host "$logPrefix 构建 $($target.GOOS)/amd64 -> $outputPath"
        go build -trimpath -ldflags '-s -w' -o $outputPath ./cmd/render-ai-news-page
        if (-not (Test-Path -LiteralPath $outputPath -PathType Leaf)) {
            throw "构建产物不存在：$outputPath"
        }
    }

    & (Join-Path $binDirectory 'render-ai-news-page-windows-amd64.exe') version
    Write-Host "$logPrefix 三平台 amd64 构建完成"
}
finally {
    if ($hadGoos) { $env:GOOS = $previousGoos } else { Remove-Item Env:GOOS -ErrorAction SilentlyContinue }
    if ($hadGoarch) { $env:GOARCH = $previousGoarch } else { Remove-Item Env:GOARCH -ErrorAction SilentlyContinue }
    Pop-Location
}
