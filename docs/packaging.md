# 打包分发

## 资源

生产包需要三个外部资源：

| 资源 | 位置 | 说明 |
|---|---|---|
| ffmpeg | `app/resources/<goos>-<goarch>/ffmpeg` | 可重定位的单文件构建 |
| 中文字体 | `app/resources/fonts/NotoSansSC-Regular.otf` | Noto Sans SC GB2312 子集，SIL OFL 1.1 |
| 应用图标 | `app/resources/icon.png` | 1024×1024，由 `icon.svg` 离屏渲染 |

准备 ffmpeg：

```sh
cd /Users/pingfury/codes/mygo-uke
MYGO_UKE_FFMPEG_SOURCE=/path/to/ffmpeg ./scripts/prepare-resources.sh
```

脚本按当前 `go env GOOS/GOARCH` 放进平台目录。发布别的架构时要分别准备
对应的 ffmpeg；`darwin/universal` 需要 `darwin-amd64` 和 `darwin-arm64`
两份。ffmpeg 构建的许可证（LGPL/GPL）要和分发方式一致，并保留相应源码
获取方式。

字体是 `NotoSansSC-Regular.otf` 的子集，只含 ASCII、常用标点和 GB2312
汉字；许可证在 `app/resources/fonts/OFL.txt`。应用启动时读取资源并注册
为 `Noto Sans SC`，找不到就回退到系统 `PingFang SC`。

## 构建

```sh
cd app
go tool mygo build -skip-dmg -sign -
```

当前机器（darwin/amd64）的产物：

```text
app/build/darwin-amd64/MyGo Uke.app
```

验证体积：

```sh
./scripts/check-bundle-size.sh
```

2026-11-22 实测：95.4 MB，低于 100 MB 分发上限。包内已包含
`Contents/Resources/ffmpeg`、`fonts/NotoSansSC-Regular.otf` 和
`AppIcon.icns`。

## 签名与公证

临时本机验证用 ad-hoc 签名即可。对外分发时在 `app/mygo.json` 配置：

```json
{
  "macos": {
    "signingIdentity": "Developer ID Application: Your Name (TEAMID)",
    "notarize": { "keychainProfile": "notarytool-profile" }
  }
}
```

然后先保存公证凭据：

```sh
xcrun notarytool store-credentials notarytool-profile \
  --apple-id you@example.com --team-id TEAMID --password app-specific-password
```

再执行 `go tool mygo build`，MyGo 会签名、公证并 staple `.app` 和 `.dmg`。
