# kos-sdk

KOS 平台项目：作者 SDK、清单结构与打包工具 — 扩展与应用作者接口。

- 设计：`tsic/kos-design` 仓的 `kos-sdk/`（范围、契约、验收、决策记录）

## 模块布局

| 目录 | 模块 | 契约 |
|---|---|---|
| `manifest/` | 清单结构：扩展声明、`App` 清单与声明校验 | `SDK-C-3`，引用 `KOSD-C-18`、`KOSD-C-73` |
| `extension/` | 扩展接口：对象类与控制器声明面 | `SDK-C-1`，引用 `KOSD-C-19`、`KOSD-C-20` |
| `app/` | 应用接口：握手、postMessage 校验与权限 | `SDK-C-1`，引用 `KOSD-C-73`、`KOSD-C-75` |
| `packaging/` | 打包工具：包结构、摘要与校验 | `SDK-C-4`，引用 `KOSD-C-17` |
| `localcheck/` | 本地一致性入口：用例集执行、摘要比对与结论 | `SDK-C-5`，引用 `CONF-AC-1`、`CONF-C-3` |
| `validate/` | 校验问题模型，供上述模块共用 | — |
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

## 应用接口

窗口应用经 `ready` / `init` 握手取得自己的短期令牌、主题与可用能力（`KOSD-C-75`）。

- `Window.Ready` 产出应用的 `ready`；`Host.Receive` 只回答来源为清单 `entry` 的 origin、发送者为所嵌入
  窗口的 `ready`，且每个窗口实例只回答一次。origin 归一化丢弃 https 默认端口 443，其余端口保留。
- `Host` 的答复是 `Delivery`：目标 origin 恒为应用 origin，SDK 不产出通配来源。
- `Window.Accept` 只接受来自控制台 origin 与父窗口、结构为 `init`、令牌未过期且绑定本应用的答复。
- 两条消息的字段集合都是封闭的：`ready` 只有 `type`；`init` 只有 `type`、`token`、`theme`、`capabilities`。
  未知消息、来源不符与多余字段一律忽略，应用没有向控制台传递 HTML 或脚本的通道。
- 可用能力 = 清单声明的权限 ∩ 该窗口已确认的权限，按固定集合顺序给出；未声明的权限不可用。
- 令牌绑定账号、应用、窗口实例、权限与到期时间，只在握手答复中出现一次。
- 接口面不含读取控制台会话的入口：`app` 包只依赖清单类型，模块不引入集群客户端或 Cookie 存储。

## 打包

打包只有一条路径，第一方与第三方共用（`SDK-C-4`，引用 `KOSD-C-17`）。工具只接收清单与作者随包声明的资产，
不读产品身份与安装来源，因此同样的清单与资产产出同样的包。

包结构固定，除下列条目外不写任何内容：

| 条目 | 内容 |
|---|---|
| `manifest.json` | 校验后的扩展声明，字段集合与 `KOSD-C-18`、`KOSD-C-73` 相同 |
| `digest` | 包摘要，`sha256:<hex>` |
| `assets/<path>` | 作者声明的 UI 资产，路径必须位于 `assets/` 之下 |

包摘要覆盖 `manifest.json` 与全部 UI 资产：按路径排序，每条以 `<名字> <长度>\n<内容>` 计入，
所以任一资产或清单变化都改变摘要，重命名资产也改变摘要。

`Pack` 在校验失败或资产不可用时返回 `*validate.InvalidError` 且不产出包：资产路径必须位于 `assets/`
之下且不得重复；`App` 的 `icon` 若指向包内资产，该资产必须随包提供。

`Verify` 是安装路径对包所做的检查：只接受上述结构，摘要必须覆盖清单与全部资产，声明仍然有效；
读取受预算约束，条目数与内容总字节数各有上限，超限的包按不可读拒绝且不返回部分内容，
调用方可用 `VerifyWithLimits` 收紧预算。`expected` 按解析后的摘要比较，大小写不影响结果。

## 本地一致性入口

`localcheck` 让作者在本机运行与 conformance 套件相同的用例集（`SDK-C-5`，引用 `CONF-AC-1`、`CONF-C-3`）。
入口复用套件的用例集与摘要判定，不另设通过标准：

- 用例集由套件提供：每条用例带所属组（能力、Provider、扩展、应用）、它验证的契约条目、判据版本与检查。
  入口不增删用例，报告按四组列出，逐条给出结果。
- 用例集摘要覆盖用例集合与每条用例的判据版本，与用例的书写顺序无关；摘要与发行版记录的摘要一致时结论可比。
- 结论取 `passed`、`failed`、`incomparable`：摘要不一致时记为不可比，同时给出运行摘要与发行版摘要，
  不记为通过或失败；可比但存在未通过或未执行用例时记为未通过。
- 未通过项与未执行项分列，未运行的用例不计入通过；单条用例失败不中止运行，其余用例照常给出结果。
- 用例集不合法（空集、组越界、缺契约归属、缺判据版本、重复 id、缺检查）时不产出报告。

## 构建与测试

```sh
make lint        # gofmt 检查 + go vet
make test        # 单元与验收用例
make acceptance  # 只跑 tests/acceptance/，-v
```

当前验收用例：`tests/acceptance/SDK-TC-1/`（P1）、`tests/acceptance/SDK-TC-2/`（P1）、
`tests/acceptance/SDK-TC-3/`（P1）、`tests/acceptance/SDK-TC-4/`（P0）、`tests/acceptance/SDK-TC-5/`（P1）、
`tests/acceptance/SDK-TC-6/`（P1）、`tests/acceptance/SDK-TC-7/`（P2）、`tests/acceptance/SDK-TC-8/`（P1）。

## 边界

- 不定义内核语义或新增扩展点；跨项目语义引用 `KOSD-C-x`，不复制判据。
- 不定义安装路径行为（属 kosd）；打包工具只保证产出的包通过安装路径对包的检查。
- 本地一致性入口只给出逐条结果、摘要与可比性，不替代 conformance 套件的结论，也不提供安装路径。
