# kos-sdk

KOS 平台项目：作者 SDK、清单结构与打包工具 — 扩展与应用作者接口。

- 设计：`tsic/kos-design` 仓的 `kos-sdk/`（范围、契约、验收、决策记录）

## 模块布局

| 目录 | 模块 | 契约 |
|---|---|---|
| `manifest/` | 清单结构：扩展声明、`App` 清单与声明校验 | `SDK-C-3`，引用 `KOSD-C-18`、`KOSD-C-73` |
| `extension/` | 扩展接口：对象类与控制器声明面 | `SDK-C-1`，引用 `KOSD-C-19`、`KOSD-C-20` |
| `validate/` | 校验问题模型，供上述两个模块共用 | — |
| `client/` | 客户端库：控制面 API 客户端与错误包络 | `SDK-C-2`，引用 `KOSD-C-12` |
| `tests/acceptance/<TC-id>/` | 验收用例，目录名取设计编号 | `03-acceptance.md` 的对应条目 |

## 清单结构

清单字段集合由框架定义，SDK 只校验与打包：不新增字段、不放宽必填项、缺项不补默认值。

扩展声明（`KOSD-C-18`）的顶层字段与契约类别一一对应：

| 字段 | 契约类别 |
|---|---|
| `id`、`version` | 标识与版本 |
| `frameworkVersion` | 目标框架 API 版本范围 |
| `contributions` | 贡献清单 |
| `dependencies` | 依赖：能力及其版本范围与必需保证、组件、框架版本 |
| `permissions` | 权限：写入范围、读取范围、控制面角色、扩展对象角色写权限 |
| `config` | 组件配置结构 |
| `status` | 状态接口；扩展对象的 status 写者与条件集 |
| `uninstallRetention` | 卸载保留语义与外部资源引用 |

`contributions` 的类别固定为十项：`objectClasses`、`controllers`、`capabilities`、`pages`、
`settings`、`catalogSources`、`discoveryAdapters`、`diagnosticAdapters`、`userPreferences`、`apps`。
`objectClasses`、`controllers`、`apps` 由本 SDK 定义声明面，其余类别的条目由作者按各自契约填写，
SDK 只做结构检查；未声明的类别不出现在产物中。

`App` 清单（`KOSD-C-73`）是独立 UI 文档声明，不是 `Page` 的变体：

- 必填：`id`、`name`、`version`、`icon`、`entry`、`window`、`permissions`。
- `window` 必填：`width`、`height`、`minWidth`、`minHeight`（像素）、`resizable`、`mode`、`defaultState`。
- 可选：`backend`（出现时声明 `type` 与 `health`）、`deepLinks`；缺席即 UI-only、无深链。
- `permissions` 可空：`null` 与 `[]` 同解，产物写 `[]`；字段缺席仍判无效。
- `entry` 是独立 origin 上的 HTTPS 地址，不带用户凭据。

`App` 权限名取固定集合（`KOSD-C-75`）：

| 契约中的权限 | 清单中的名字 |
|---|---|
| 通知 | `notifications` |
| 剪贴板读 | `clipboard-read` |
| 剪贴板写 | `clipboard-write` |
| 文件选择 | `file-selection` |
| 外部链接 | `external-link` |

`ValidateExtensionManifest` 与 `ValidateAppManifest` 返回清单或问题列表；问题逐条给出路径、错误码
与说明，指出缺失项（`missing-field`）、越界项（`invalid-value`、`permission-not-in-set`）或未定义字段
（`unknown-field`）。失败结果不携带清单，打包工具不得从失败结果产出可安装工件。

## 扩展接口

- `DeclareObjectClass`：扩展对象类声明。对象类属于扩展自己的 API 组，不得使用内核十类 kind
  （`KOSD-C-01`、`KOSD-C-19`）。
- `DeclareController`：扩展控制器声明。写入范围按 API 组、kind 与动词给出，不得包含内核 kind；
  动词取 `get`、`list`、`watch`、`create`、`update`、`patch`、`delete`（`KOSD-C-20`）。
- 两个函数在非法输入上返回 `InvalidError`，其 `Issues` 与校验结果同构。

## 客户端

客户端是控制面 API 的作者侧出口，只做请求构造与响应解码：

- 错误包络的字段集合原样交给作者，错误代码与失败阶段不改写，可重试标记保留。
- 分页与过滤参数由作者显式给出；不隐式翻页，也不合并多页结果。
- 不缓存写请求结果，不替作者重试。

## 构建与测试

```sh
make lint        # gofmt 检查 + go vet
make test        # 单元与验收用例
make acceptance  # 只跑 tests/acceptance/，-v
```

当前验收用例：`tests/acceptance/SDK-TC-2/`（P1）、`tests/acceptance/SDK-TC-4/`（P0）、
`tests/acceptance/SDK-TC-5/`（P1）。

## 边界

- 不定义内核语义或新增扩展点；跨项目语义引用 `KOSD-C-x`，不复制判据。
- 打包路径与包摘要（`SDK-C-4`）不在本模块，另行交付；本模块只到清单可校验、声明被判定有效或无效。
